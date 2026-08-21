# syntax=docker/dockerfile:1

FROM golang:1.25.12-trixie AS build
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/fakeollama ./tools/fakeollama

FROM scratch
COPY --from=build /out/fakeollama /fakeollama
USER 65532:65532
EXPOSE 11434
ENTRYPOINT ["/fakeollama"]
