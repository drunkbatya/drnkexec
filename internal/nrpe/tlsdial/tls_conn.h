#ifndef DRNKEXEC_TLS_CONN_H
#define DRNKEXEC_TLS_CONN_H

#include <stddef.h>
#include <sys/types.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct nrpe_ssl_conn nrpe_ssl_conn;

int nrpe_ssl_connect(const char *host, const char *port, long timeout_ms, nrpe_ssl_conn **out, char **err);
int nrpe_ssl_set_timeout(nrpe_ssl_conn *conn, long timeout_ms, char **err);
long nrpe_ssl_current_timeout(nrpe_ssl_conn *conn);
ssize_t nrpe_ssl_read(nrpe_ssl_conn *conn, void *buf, size_t len, char **err);
ssize_t nrpe_ssl_write(nrpe_ssl_conn *conn, const void *buf, size_t len, char **err);
void nrpe_ssl_close(nrpe_ssl_conn *conn);

#ifdef __cplusplus
}
#endif

#endif
