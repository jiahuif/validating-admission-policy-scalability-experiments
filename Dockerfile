FROM golang:bookworm AS builder

ADD . /src
WORKDIR /src/test
RUN go test -c -o /test

FROM gcr.io/distroless/base-nossl-debian12
COPY --from=builder /test /bin/test
ADD testdata/ /testdata/
ENTRYPOINT ["/bin/test", "-test.v"]
