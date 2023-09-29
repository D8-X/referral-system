package utils

import "time"

type DbReferralStruct struct {
	Parent    string
	Child     string
	PassOn    float32
	CreatedOn time.Time
}
