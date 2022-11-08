IEC 62056-21 communication protocol implementation.
====

[![Coverage Status](https://coveralls.io/repos/github/srgsf/iec62056.golang/badge.svg)](https://coveralls.io/github/srgsf/iec62056.golang)
[![lint and test](https://github.com/srgsf/iec62056.golang/actions/workflows/golint-ci.yaml/badge.svg)](https://github.com/srgsf/iec62056.golang/actions/workflows/golint-ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/srgsf/iec62056.golang)](https://goreportcard.com/report/github.com/srgsf/iec62056.golang)

This is a golang wrapper for tariff devices communication protocol.
This client requires rs485 to Ethernet converter for connection and operates only via TCP.

Not implemented:
- Protocol Mode E.
- Partial data block reading.

Communication protocol details can be found [here](iec62056-21.pdf)
