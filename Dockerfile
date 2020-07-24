FROM golang:1.14-alpine AS builder

ADD ./ /src
WORKDIR /src
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o opa-sidecar .

FROM alpine:3.11
EXPOSE 5151
ENV ADDRESS=localhost:5151
RUN apk --no-cache add curl
RUN curl -L -o /usr/bin/opa https://openpolicyagent.org/downloads/latest/opa_linux_amd64 && chmod +x /usr/bin/opa
COPY --from=builder /src/opa-sidecar /usr/bin/opa-sidecar
CMD ["/usr/bin/opa-sidecar"]
