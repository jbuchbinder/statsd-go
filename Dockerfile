FROM golang

RUN go get github.com/jbuchbinder/go-gmetric/gmetric
ADD . /go/src/github.com/jbuchbinder/statsd-go
RUN go install github.com/jbuchbinder/statsd-go
EXPOSE 8125/udp
ENTRYPOINT ["/go/bin/statsd-go"]

