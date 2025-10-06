FROM golang:1.24.7-alpine3.22

WORKDIR /usr/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -v -o /usr/local/bin/app 

EXPOSE 3000
CMD ["app"]