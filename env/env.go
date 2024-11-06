package env

type Args struct {
	Test               *bool
	NoWow              *bool
	Verbose            *bool
	Imuon              *bool
	Speedon            *bool
	Diron              *bool
	Rainon             *bool
	WindEnabled        *bool
	AtmosphericEnabled *bool
	RainEnabled        *bool
	Humidity           *bool
	WowSiteID          string
	WowPin             string
}
