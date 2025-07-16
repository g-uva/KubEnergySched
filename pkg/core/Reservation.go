package core

import "time"

type Reservation struct {
    endTime      time.Time
    cpuReserved  float64
    memReserved  float64
}