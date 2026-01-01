#include "tls_conn.h"

#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <poll.h>
#include <pthread.h>
#include <stdarg.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/time.h>
#include <sys/types.h>
#include <unistd.h>

#include <openssl/err.h>
#include <openssl/rand.h>
#include <openssl/ssl.h>

struct nrpe_ssl_conn {
    SSL_CTX *ctx;
    SSL *ssl;
    int fd;
    long timeout_ms;
};

static pthread_mutex_t init_mutex = PTHREAD_MUTEX_INITIALIZER;
static bool ssl_initialized = false;

static void nrpe_tls_init(void) {
    pthread_mutex_lock(&init_mutex);
    if (!ssl_initialized) {
        SSL_library_init();
        SSL_load_error_strings();
        OpenSSL_add_all_algorithms();
        ssl_initialized = true;
    }
    pthread_mutex_unlock(&init_mutex);
}

static int set_error(char **err, const char *fmt, ...) {
    if (err == NULL) {
        return -1;
    }
    va_list ap;
    va_start(ap, fmt);
    char buf[512];
    vsnprintf(buf, sizeof(buf), fmt, ap);
    va_end(ap);
    *err = strdup(buf);
    return -1;
}

static int set_nonblock(int fd) {
    int flags = fcntl(fd, F_GETFL, 0);
    if (flags == -1) {
        return -1;
    }
    if (fcntl(fd, F_SETFL, flags | O_NONBLOCK) == -1) {
        return -1;
    }
    return 0;
}

static int wait_for_connect(int fd, long timeout_ms) {
    struct pollfd pfd;
    pfd.fd = fd;
    pfd.events = POLLOUT;
    int ret = poll(&pfd, 1, (int)timeout_ms);
    if (ret == 0) {
        errno = ETIMEDOUT;
        return -1;
    }
    if (ret < 0) {
        return -1;
    }
    int err = 0;
    socklen_t len = sizeof(err);
    if (getsockopt(fd, SOL_SOCKET, SO_ERROR, &err, &len) < 0) {
        return -1;
    }
    if (err != 0) {
        errno = err;
        return -1;
    }
    return 0;
}

static int connect_with_timeout(const char *host, const char *port, long timeout_ms, int *out_fd, char **err) {
    struct addrinfo hints;
    struct addrinfo *res = NULL, *rp;
    memset(&hints, 0, sizeof(hints));
    hints.ai_socktype = SOCK_STREAM;
    hints.ai_family = AF_UNSPEC;

    int gai = getaddrinfo(host, port, &hints, &res);
    if (gai != 0) {
        return set_error(err, "resolve %s:%s failed: %s", host, port, gai_strerror(gai));
    }
    int fd = -1;
    for (rp = res; rp != NULL; rp = rp->ai_next) {
        fd = socket(rp->ai_family, rp->ai_socktype, rp->ai_protocol);
        if (fd < 0)
            continue;
        if (set_nonblock(fd) != 0) {
            close(fd);
            fd = -1;
            continue;
        }
        if (connect(fd, rp->ai_addr, rp->ai_addrlen) < 0) {
            if (errno != EINPROGRESS) {
                close(fd);
                fd = -1;
                continue;
            }
            if (wait_for_connect(fd, timeout_ms) != 0) {
                close(fd);
                fd = -1;
                continue;
            }
        }
        int flags = fcntl(fd, F_GETFL, 0);
        if (flags != -1) {
            fcntl(fd, F_SETFL, flags & (~O_NONBLOCK));
        }
        break;
    }
    freeaddrinfo(res);
    if (fd < 0) {
        return set_error(err, "connect %s:%s failed: %s", host, port, strerror(errno));
    }
    *out_fd = fd;
    return 0;
}

static int set_sock_timeout(int fd, long timeout_ms) {
    struct timeval tv;
    tv.tv_sec = timeout_ms / 1000;
    tv.tv_usec = (timeout_ms % 1000) * 1000;
    if (setsockopt(fd, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv)) != 0)
        return -1;
    if (setsockopt(fd, SOL_SOCKET, SO_SNDTIMEO, &tv, sizeof(tv)) != 0)
        return -1;
    return 0;
}

static int configure_ctx(SSL_CTX *ctx, char **err) {
    long opts = SSL_OP_ALL;
#ifdef SSL_OP_NO_COMPRESSION
    opts |= SSL_OP_NO_COMPRESSION;
#endif
    SSL_CTX_set_options(ctx, opts);
#ifdef SSL_CTX_set_security_level
    SSL_CTX_set_security_level(ctx, 0);
#endif
    if (SSL_CTX_set_cipher_list(ctx, "ALL:!MD5:@STRENGTH:@SECLEVEL=0") != 1) {
        unsigned long code = ERR_get_error();
        return set_error(err, "set cipher list failed: %s", ERR_error_string(code, NULL));
    }
    SSL_CTX_set_verify(ctx, SSL_VERIFY_NONE, NULL);
    return 0;
}

int nrpe_ssl_connect(const char *host, const char *port, long timeout_ms, nrpe_ssl_conn **out, char **err) {
    if (host == NULL || port == NULL || out == NULL)
        return set_error(err, "invalid arguments");
    nrpe_tls_init();
    int fd;
    if (connect_with_timeout(host, port, timeout_ms, &fd, err) != 0)
        return -1;
    if (timeout_ms > 0 && set_sock_timeout(fd, timeout_ms) != 0) {
        close(fd);
        return set_error(err, "setsockopt failed: %s", strerror(errno));
    }
    SSL_CTX *ctx = SSL_CTX_new(TLS_client_method());
    if (ctx == NULL) {
        close(fd);
        return set_error(err, "SSL_CTX_new failed");
    }
    if (configure_ctx(ctx, err) != 0) {
        SSL_CTX_free(ctx);
        close(fd);
        return -1;
    }
    SSL *ssl = SSL_new(ctx);
    if (ssl == NULL) {
        SSL_CTX_free(ctx);
        close(fd);
        return set_error(err, "SSL_new failed");
    }
    if (SSL_set_fd(ssl, fd) != 1) {
        SSL_free(ssl);
        SSL_CTX_free(ctx);
        close(fd);
        return set_error(err, "SSL_set_fd failed");
    }
    if (SSL_connect(ssl) != 1) {
        unsigned long code = ERR_get_error();
        SSL_free(ssl);
        SSL_CTX_free(ctx);
        close(fd);
        return set_error(err, "SSL_connect failed: %s", ERR_error_string(code, NULL));
    }
    nrpe_ssl_conn *conn = calloc(1, sizeof(*conn));
    if (conn == NULL) {
        SSL_free(ssl);
        SSL_CTX_free(ctx);
        close(fd);
        return set_error(err, "out of memory");
    }
    conn->ctx = ctx;
    conn->ssl = ssl;
    conn->fd = fd;
    conn->timeout_ms = timeout_ms;
    *out = conn;
    return 0;
}

int nrpe_ssl_set_timeout(nrpe_ssl_conn *conn, long timeout_ms, char **err) {
    if (conn == NULL)
        return set_error(err, "invalid connection");
    if (timeout_ms <= 0)
        timeout_ms = conn->timeout_ms;
    if (timeout_ms <= 0)
        timeout_ms = 10000;
    if (set_sock_timeout(conn->fd, timeout_ms) != 0)
        return set_error(err, "setsockopt failed: %s", strerror(errno));
    conn->timeout_ms = timeout_ms;
    return 0;
}

long nrpe_ssl_current_timeout(nrpe_ssl_conn *conn) {
    if (conn == NULL)
        return 0;
    return conn->timeout_ms;
}

static ssize_t translate_io_result(ssize_t ret, int ssl_err, char **err) {
    if (ret > 0)
        return ret;
    if (ret == 0 && ssl_err == SSL_ERROR_ZERO_RETURN)
        return 0;
    if (ssl_err == SSL_ERROR_WANT_READ || ssl_err == SSL_ERROR_WANT_WRITE) {
        return set_error(err, "operation timed out");
    }
    if (ssl_err == SSL_ERROR_SYSCALL) {
        return set_error(err, "ssl syscall failed: %s", strerror(errno));
    }
    unsigned long code = ERR_get_error();
    return set_error(err, "SSL IO failed: %s", ERR_error_string(code, NULL));
}

ssize_t nrpe_ssl_read(nrpe_ssl_conn *conn, void *buf, size_t len, char **err) {
    if (conn == NULL)
        return set_error(err, "invalid connection");
    int ret = SSL_read(conn->ssl, buf, (int)len);
    int s_err = SSL_get_error(conn->ssl, ret);
    return translate_io_result(ret, s_err, err);
}

ssize_t nrpe_ssl_write(nrpe_ssl_conn *conn, const void *buf, size_t len, char **err) {
    if (conn == NULL)
        return set_error(err, "invalid connection");
    int ret = SSL_write(conn->ssl, buf, (int)len);
    int s_err = SSL_get_error(conn->ssl, ret);
    return translate_io_result(ret, s_err, err);
}

void nrpe_ssl_close(nrpe_ssl_conn *conn) {
    if (conn == NULL)
        return;
    if (conn->ssl)
        SSL_shutdown(conn->ssl);
    if (conn->ssl)
        SSL_free(conn->ssl);
    if (conn->ctx)
        SSL_CTX_free(conn->ctx);
    if (conn->fd >= 0)
        close(conn->fd);
    free(conn);
}
