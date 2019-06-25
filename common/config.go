package common

import "time"

var (
	TTL = 120 * time.Second
	MTU = 1460
	DIAL_TIEMOUT = 10 * time.Second
	WRITE_TIMEOUT = 10*time.Second
	READ_TIMEOUT = 10*time.Second

	KEEP_ALIVE_DURATION = 50*time.Millisecond
)