# Phase 1 deployable image for the LiveTemplate patterns showcase.
# Targets fly.io as `lt-patterns` so the docs site's external-app router
# can proxy /patterns/ to it. Generic enough to run anywhere.
#
# Source is fetched at build time so the build context can be just the
# patterns/ directory (makes flyctl deploys self-contained per app).

ARG EXAMPLES_REF=docs/phase-1-patterns-deploy

# ---- Build stage ----
FROM golang:1.26-alpine AS go-builder
ARG EXAMPLES_REF
RUN apk add --no-cache git ca-certificates
ENV GOTOOLCHAIN=auto
WORKDIR /src
RUN git clone --depth=1 --branch=${EXAMPLES_REF} https://github.com/livetemplate/examples.git .
WORKDIR /src/patterns
RUN go mod download -C ..
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /out/patterns .

# ---- Runtime stage ----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 1000 patterns
WORKDIR /app
COPY --from=go-builder /out/patterns /usr/local/bin/patterns
# Templates are loaded by livetemplate.WithParseFiles at runtime as
# relative paths (e.g. "templates/layout.tmpl"), so cwd must contain
# the templates/ directory at process start.
COPY --from=go-builder /src/patterns/templates /app/templates
RUN chown -R patterns:patterns /app
USER patterns
EXPOSE 8080
ENV PORT=8080
CMD ["patterns"]
