package reddit

import (
	"strconv"
	"time"
)

type Timestamp struct {
	time.Time
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte(`false`), nil
	}

	parsed := t.Time.Format(time.RFC3339)
	return []byte(`"` + parsed + `"`), nil
}

func (t *Timestamp) UnmarshalJSON(data []byte) (err error) {
	str := string(data)

	// "edited" for posts and comments is either false, or a timestamp.
	if str == "false" {
		return
	}

	f, err := strconv.ParseFloat(str, 64)
	if err == nil {
		t.Time = time.Unix(int64(f), 0).UTC()
	} else {
		t.Time, err = time.Parse(`"`+time.RFC3339+`"`, str)
	}
	return
}

func (t Timestamp) Equal(u Timestamp) bool {
	return t.Time.Equal(u.Time)
}
