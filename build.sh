#!/bin/bash

# raspberry pi
GOOS=linux GOARCH=arm GOARM=6 go build ./cmd/ddnscf
