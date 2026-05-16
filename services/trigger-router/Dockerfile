FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/trigger-router ./cmd/trigger-router

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/trigger-router /trigger-router
EXPOSE 8097
USER nonroot:nonroot
ENTRYPOINT ["/trigger-router"]
