package utils

import (
	"fmt"
	"math/big"
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
	schedule = "* * * * *"
	if IsValidPaymentSchedule(schedule) {
		t.Errorf("IsValidPaymentSchedule(" + schedule + ") = true, expected false")
	}
}

func TestAbdkQuo(t *testing.T) {
	strNumber := "226894952106627484" // Replace with your string number
	myBigInt := new(big.Int)

	// SetString parses the string and sets the value of the big.Int
	myBigInt.SetString(strNumber, 10)
	// want 12299999999999999
	v2 := ABDKToDecN(myBigInt, 18)
	fmt.Println(v2.String())
	am := DecNTimesFloat(v2, 0.9999, 18)
	fmt.Println(am.String())
}

func TestRatio(t *testing.T) {
	x, _ := new(big.Int).SetString("12345678901234567890", 10)
	y, _ := new(big.Int).SetString("9876543210987654321", 10)
	r := Ratio(x, y)
	fmt.Println(r) //~1.25
}
