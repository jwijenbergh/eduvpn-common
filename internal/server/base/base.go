package base

import (
	"time"

	"github.com/eduvpn/eduvpn-common/internal/http"
	"github.com/eduvpn/eduvpn-common/internal/server/endpoints"
	"github.com/eduvpn/eduvpn-common/internal/server/profile"
	"github.com/eduvpn/eduvpn-common/types/server"
)

// Base is the base type for servers.
type Base struct {
	URL            string              `json:"base_url"`
	DisplayName    map[string]string   `json:"display_name"`
	SupportContact []string            `json:"support_contact"`
	Endpoints      endpoints.Endpoints `json:"endpoints"`
	Profiles       profile.Info        `json:"profiles"`
	StartTime      time.Time           `json:"start_time"`
	EndTime        time.Time           `json:"expire_time"`
	Type           server.Type         `json:"server_type"`
	HTTPClient     *http.Client        `json:"-"`
}

// RenewButtonTime returns the time when the renew button should be shown for the server
// Implemented according to: https://github.com/eduvpn/documentation/blob/cdf4d054f7652d74e4192494e8bb0e21040e46ac/API.md#session-expiry
func (b *Base) RenewButtonTime() int64 {
	d := b.EndTime.Sub(b.StartTime)

	// If the time is less than 24 hours (a day left), show it when 30 minutes have passed or on expired if less than 30 minutes
	dayl := time.Duration(24 * time.Hour)
	if d < dayl {
		// Get the minimum time to add, 30 minutes or on expired
		m := time.Duration(30 * time.Minute)
		// The total delta time is larger, return that we should show the button after 30 minutes
		if d > m {
			return b.StartTime.Add(30 * time.Minute).Unix()
		}
		// Just show it on expired
		return b.StartTime.Add(d).Unix()
	}

	// Else just show it when 24 hours is left
	// This is the delta minus 24 hours left as that's how long it takes for a day to be left in the expiry
	// We thus add this to the start time
	tillDay := d - dayl
	t := b.StartTime.Add(tillDay)
	return t.Unix()
}

func (b *Base) CountdownTime() int64 {
	d := b.EndTime.Sub(b.StartTime)

	dayl := time.Duration(24 * time.Hour)

	// This is just the last 24 hours
	// if less than or equal to 24 hours, immediately
	if d <= dayl {
		return b.StartTime.Unix()
	}

	tillDay := d - dayl
	t := b.StartTime.Add(tillDay)
	return t.Unix()
}

func (b *Base) NotificationTimes() []int64 {
	last := []time.Duration{
		time.Duration(0),
		time.Duration(1 * time.Hour),
		time.Duration(2 * time.Hour),
		time.Duration(4 * time.Hour),
	}

	var t []int64

	d := b.EndTime.Sub(b.StartTime)
	for _, l := range last {
		// If the notification remaining time is more than the total delta, continue
		if l > d {
			continue
		}
		// calculating the time till a notification must happen
		tillN := d - l
		// Get absolute time when this notification must be shown by adding the delta
		c := b.StartTime.Add(tillN)
		t = append(t, c.Unix())
	}
	return t
}
