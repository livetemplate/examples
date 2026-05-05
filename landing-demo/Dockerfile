# Deployable image for the LiveTemplate landing-demo. The docs site
# proxies same-origin to this app so the home page can iframe a real
# LiveTemplate counter without cross-origin or chrome friction.

ARG EXAMPLES_REF=main

# ---- Build stage ----
FROM golang:1.26-alpine AS go-builder
ARG EXAMPLES_REF
RUN apk add --no-cache git ca-certificates
ENV GOTOOLCHAIN=auto
WORKDIR /src
RUN git clone --depth=1 --branch=${EXAMPLES_REF} https://github.com/livetemplate/examples.git .
# Fail fast (vs. silently shipping a stale build) if the requested ref
# does not contain landing-demo. Common cause: deploying before this app
# is merged to main without overriding --build-arg EXAMPLES_REF=<branch>.
RUN test -d /src/landing-demo || (echo "ERROR: landing-demo/ not found at ref '${EXAMPLES_REF}'. If deploying from a branch, pass --build-arg EXAMPLES_REF=<branch-name>." && exit 1)
WORKDIR /src/landing-demo
RUN go mod download -C ..
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/landing-demo .

# ---- Runtime stage ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 demo
WORKDIR /app
COPY --from=go-builder /out/landing-demo /usr/local/bin/landing-demo
# counter.tmpl is loaded via livetemplate.WithParseFiles at runtime as
# a relative path, so cwd must contain it at process start.
COPY --from=go-builder /src/landing-demo/counter.tmpl /app/counter.tmpl
RUN chown -R demo:demo /app
USER demo
EXPOSE 8080
ENV PORT=8080
CMD ["landing-demo"]
