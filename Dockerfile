FROM golang:1.19 as build

RUN update-ca-certificates

WORKDIR /go/src/app

COPY go.mod .
COPY go.sum .

RUN go mod download
RUN go mod verify

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/rosterd ./

FROM gcr.io/distroless/static

COPY --from=build /go/bin/rosterd /go/bin/rosterd

ENTRYPOINT ["/go/bin/rosterd"]
