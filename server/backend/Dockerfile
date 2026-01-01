FROM golang:1.25.5-alpine3.23 AS builder
WORKDIR /app

ARG GIT_COMMIT
RUN test -n "$GIT_COMMIT"
ARG GIT_BRANCH
RUN test -n "$GIT_BRANCH"
ARG GIT_BRANCH_NUM
RUN test -n "$GIT_BRANCH_NUM"
ARG BUILD_DATE
RUN test -n "$BUILD_DATE"
ARG BUILD_TIME
RUN test -n "$BUILD_TIME"
ARG BUILD_VERSION
RUN test -n "$BUILD_VERSION"

RUN apk add --no-cache build-base openssl-dev

ENV CGO_ENABLED=1
ENV PATH=/go/bin:$PATH

COPY src/go.mod src/go.sum ./src/
WORKDIR /app/src
RUN go mod download
RUN go install github.com/swaggo/swag/cmd/swag@latest

COPY src .
RUN go generate ./cmd/drnkexec

RUN LDFLAGS="-X github.com/drunkbatya/drnkexec/internal/version.gitCommit=${GIT_COMMIT} \
    -X github.com/drunkbatya/drnkexec/internal/version.gitBranch=${GIT_BRANCH} \
    -X github.com/drunkbatya/drnkexec/internal/version.gitBranchNum=${GIT_BRANCH_NUM} \
    -X github.com/drunkbatya/drnkexec/internal/version.buildDate=${BUILD_DATE} \
    -X github.com/drunkbatya/drnkexec/internal/version.buildTime=${BUILD_TIME} \
    -X github.com/drunkbatya/drnkexec/internal/version.buildVersion=${BUILD_VERSION}" && \
    go build -ldflags "$LDFLAGS" -o /app/bin/drnkexec ./cmd/drnkexec

FROM alpine:3.23
WORKDIR /app
#RUN apk add --no-cache openssl ca-certificates
COPY --from=builder /app/bin/drnkexec /usr/local/bin/drnkexec
ENTRYPOINT ["/usr/local/bin/drnkexec"]
