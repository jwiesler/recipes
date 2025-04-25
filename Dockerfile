FROM docker.io/rust:alpine AS builder
RUN apk add musl-dev
WORKDIR /src
COPY . .
RUN cargo build --release

FROM docker.io/alpine
COPY --from=builder /src/target/release/recipes /usr/local/bin/recipes
COPY templates /templates
COPY static /static

USER 2000:2000
EXPOSE 4200

ENTRYPOINT [ "/usr/local/bin/recipes" ]
