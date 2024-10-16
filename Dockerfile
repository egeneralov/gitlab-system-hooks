FROM golang:1.23.1-alpine

RUN apk add --no-cache ca-certificates

ENV \
  GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /go/src/github.com/egeneralov/gitlab-system-hooks
ADD go.mod go.sum /go/src/github.com/egeneralov/gitlab-system-hooks/
RUN go mod download

ADD . .

RUN go build -v -installsuffix cgo -ldflags="-w -s" -o /go/bin/gitlab-system-hooks .


FROM alpine

RUN apk add --no-cache ca-certificates
USER nobody
ENV PATH='/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
CMD /go/bin/gitlab-system-hooks

COPY --from=0 /go/bin /go/bin


