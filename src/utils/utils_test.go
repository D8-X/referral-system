package utils

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestPaymentSchedule(t *testing.T) {
	schedule := "0 14 * * 2"
	if !IsValidPaymentSchedule(schedule) {
		t.Errorf("IsValidPaymentSchedule(" + schedule + ") = false, expected true")
	}
	currentTime := time.Now()
	currentHour := currentTime.Hour()
	currentMinute := currentTime.Minute()
	fmt.Printf("Current Time: %02d:%02d\n", currentHour, currentMinute)
	schedule = strconv.Itoa(currentMinute-1) + " " +
		strconv.Itoa(currentHour) + " * * *"
	prevTime := PrevPaymentSchedule(schedule)
	nxtTime := NextPaymentSchedule(schedule)
	fmt.Println("prev=", prevTime)
	fmt.Println("now =", currentTime)
	fmt.Println("next=", nxtTime)
	if !(currentTime.After(prevTime) && nxtTime.After(currentTime)) {
		t.Errorf("unexpected order")
	}
}
