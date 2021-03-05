# STATSD-GO

[![Build Status](https://secure.travis-ci.org/jbuchbinder/statsd-go.png)](http://travis-ci.org/jbuchbinder/statsd-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/jbuchbinder/statsd-go)](https://goreportcard.com/report/github.com/jbuchbinder/statsd-go)
[![GoDoc](https://godoc.org/github.com/jbuchbinder/statsd-go?status.png)](https://godoc.org/github.com/jbuchbinder/statsd-go)

Port of Etsy's statsd, written in Go.

This was forked from https://github.com/amir/gographite to provide
Ganglia submission support.

## USAGE

```
Usage of statsd-go:
  -address=":8125": UDP service address
  -debug=false: Debug mode
  -flush-interval=10: Flush interval
  -ganglia="localhost": Ganglia gmond servers, comma separated
  -ganglia-group="statsd": Ganglia metric group name
  -ganglia-port=8649: Ganglia gmond service port
  -ganglia-spoof-host="": Ganglia gmond spoof host string
  -graphite="": Graphite service address (example: 'localhost:2003')
  -percent-threshold=90: Threshold percent
```

