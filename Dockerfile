
# Build the frontend
FROM node:16 as uibuild
ARG CONFIGURATION="production"

WORKDIR /app/ui

COPY ui/.npmrc ui/package.json ui/package-lock.json ./
RUN --mount=type=secret,id=github_token \
  echo "//npm.pkg.github.com/:_authToken=$(cat /run/secrets/github_token)" >> ./.npmrc && npm install && rm .npmrc

RUN npx browserslist@latest --update-db

COPY ./ui .
RUN npm run build -- --configuration $CONFIGURATION && rm -r .angular/cache node_modules

# Build the frontend
FROM node:16 as mailbuild

WORKDIR /app/mails

COPY mails/package.json mails/package-lock.json ./
RUN npm install

COPY ./mails .
RUN npm run build

# Build the go binary
FROM golang:1.21 as gobuild
 
RUN update-ca-certificates
 
WORKDIR /go/src/app

COPY go.mod .
COPY go.sum .

RUN go mod download
RUN go mod verify

COPY . .
COPY --from=uibuild /app/ui/dist/ui /go/src/app/ui/dist/ui
COPY --from=mailbuild /app/mails/dist /go/src/app/mails/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /go/bin/rosterd ./

FROM gcr.io/distroless/static
EXPOSE 8080

COPY --from=gobuild /go/bin/rosterd /go/bin/rosterd
#COPY ./rosterd /go/bin/rosterd

ENTRYPOINT ["/go/bin/rosterd"]
