package i18n

import (
	"fmt"
	"time"
)

// 相对时间（"18 秒前"）。git 的 %ar / committerdate:relative 只会输出英文，
// 所以改为让 git 给时间戳，在这里按当前语言格式化。
// 分档规则照搬 git（date.c 的 show_date_relative），数值和 git 显示的一致。

type relUnit int

const (
	uSec relUnit = iota
	uMin
	uHour
	uDay
	uWeek
	uMonth
	uYear
)

// relParts 返回 (数量, 单位)；超过一年不满五年时 months > 0 表示「n 年 m 个月」。
// future 为 true 表示时间在将来（时钟不准等）。
func relParts(t, now time.Time) (n int, u relUnit, months int, future bool) {
	d := int64(now.Sub(t) / time.Second)
	if d < 0 {
		return 0, 0, 0, true
	}
	switch {
	case d < 90:
		return int(d), uSec, 0, false
	case d < 90*60:
		return int((d + 30) / 60), uMin, 0, false
	case d < 36*3600:
		return int((d + 30*60) / 3600), uHour, 0, false
	}
	days := (d + 12*3600) / 86400
	switch {
	case days < 14:
		return int(days), uDay, 0, false
	case days < 70:
		return int((days + 3) / 7), uWeek, 0, false
	case days < 365:
		return int((days + 15) / 30), uMonth, 0, false
	case days < 1825:
		total := (days*12*2 + 365) / (365 * 2)
		return int(total / 12), uYear, int(total % 12), false
	}
	return int((days + 183) / 365), uYear, 0, false
}

// 每种语言的单位写法：[单数, 复数]，只有英文需要区分。
var relWords = map[Lang][7][2]string{
	ZhCN: {{"秒", "秒"}, {"分钟", "分钟"}, {"小时", "小时"}, {"天", "天"}, {"周", "周"}, {"个月", "个月"}, {"年", "年"}},
	ZhTW: {{"秒", "秒"}, {"分鐘", "分鐘"}, {"小時", "小時"}, {"天", "天"}, {"週", "週"}, {"個月", "個月"}, {"年", "年"}},
	En:   {{"second", "seconds"}, {"minute", "minutes"}, {"hour", "hours"}, {"day", "days"}, {"week", "weeks"}, {"month", "months"}, {"year", "years"}},
	Ja:   {{"秒", "秒"}, {"分", "分"}, {"時間", "時間"}, {"日", "日"}, {"週間", "週間"}, {"か月", "か月"}, {"年", "年"}},
	Ko:   {{"초", "초"}, {"분", "분"}, {"시간", "시간"}, {"일", "일"}, {"주", "주"}, {"개월", "개월"}, {"년", "년"}},
}

func word(l Lang, u relUnit, n int) string {
	w := relWords[l][u]
	if n == 1 {
		return w[0]
	}
	return w[1]
}

// Ago 返回 t 距今的相对时间，比如 "18 秒前"、"3 hours ago"、"1年3か月前"。
func Ago(t time.Time) string { return agoAt(t, time.Now(), current) }

func agoAt(t, now time.Time, l Lang) string {
	if _, ok := relWords[l]; !ok {
		l = ZhCN
	}
	n, u, months, future := relParts(t, now)
	if future {
		switch l {
		case En:
			return "in the future"
		case Ko:
			return "미래"
		}
		return "未来"
	}
	// qty 拼出「数量 + 单位」，各语言的空格习惯不同
	qty := func(n int, u relUnit) string {
		switch l {
		case En, ZhCN, ZhTW:
			return fmt.Sprintf("%d %s", n, word(l, u, n))
		}
		return fmt.Sprintf("%d%s", n, word(l, u, n)) // 日语、韩语数字和单位连写
	}
	s := qty(n, u)
	if months > 0 {
		switch l {
		case En:
			s += ", " + qty(months, uMonth)
		case Ja:
			s += qty(months, uMonth) // 1年3か月前
		default:
			s += " " + qty(months, uMonth)
		}
	}
	switch l {
	case En:
		return s + " ago"
	case Ko:
		return s + " 전"
	}
	return s + "前" // 中文、日语

}
