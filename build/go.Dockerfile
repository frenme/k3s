FROM golang:1.26-alpine AS build
ARG SERVICE
ARG VERSION=dev
ARG COMMIT=none
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/egor/k3s-base/internal/platform/version.Version=${VERSION} -X github.com/egor/k3s-base/internal/platform/version.Commit=${COMMIT}" -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app"]
