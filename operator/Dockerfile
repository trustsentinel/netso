# Build the netso operator (controller-runtime). Build context is the operator/
# module directory.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/netso-operator .

FROM alpine:3.20
COPY --from=build /out/netso-operator /usr/local/bin/netso-operator
ENTRYPOINT ["/usr/local/bin/netso-operator"]
