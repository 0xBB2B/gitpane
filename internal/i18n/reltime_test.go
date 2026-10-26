package i18n

import (
	"testing"
	"time"
)

func TestAgo(t *testing.T) {
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	day := 24 * time.Hour
	cases := []struct {
		t    time.Time
		l    Lang
		want string
	}{
		{ago(18 * time.Second), ZhCN, "18 秒前"},
		{ago(18 * time.Second), En, "18 seconds ago"},
		{ago(1 * time.Second), En, "1 second ago"},
		{ago(5 * time.Minute), ZhTW, "5 分鐘前"},
		{ago(3 * time.Hour), Ja, "3時間前"},
		{ago(3 * time.Hour), Ko, "3시간 전"},
		{ago(2 * day), ZhCN, "2 天前"},
		{ago(21 * day), En, "3 weeks ago"},
		{ago(120 * day), Ja, "4か月前"},
		{ago(456 * day), ZhCN, "1 年 3 个月前"},
		{ago(456 * day), En, "1 year, 3 months ago"},
		{ago(456 * day), Ja, "1年3か月前"},
		{ago(456 * day), Ko, "1년 3개월 전"},
		{ago(730 * day), En, "2 years ago"},
		{ago(3000 * day), ZhCN, "8 年前"},
		{now.Add(time.Hour), En, "in the future"},
		{ago(18 * time.Second), Lang("xx"), "18 秒前"},
	}
	for _, c := range cases {
		if got := agoAt(c.t, now, c.l); got != c.want {
			t.Errorf("%s %v: got %q, want %q", c.l, now.Sub(c.t), got, c.want)
		}
	}
}
