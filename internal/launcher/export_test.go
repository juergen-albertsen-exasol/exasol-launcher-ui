package launcher

import "time"

func SetClock(client *Client, now func() time.Time) {
	client.now = now
}
