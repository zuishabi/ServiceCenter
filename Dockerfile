FROM golang:1.24 AS build-stage

WORKDIR /ServiceCenter

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN ls -la
RUN go build -o Server main.go


# Deploy the application binary into a lean image
FROM debian AS build-release-stage

WORKDIR /

COPY --from=build-stage /ServiceCenter/Server /Server

EXPOSE 9999

USER nobody

ENTRYPOINT ["/Server"]