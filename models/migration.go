package models

import "time"

type Migration struct {
	Id        int64
	Version   string
	CreatedAt time.Time
}

type Migrations []*Migration
