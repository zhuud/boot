package util

import (
	"strconv"
	"strings"
	"time"
)

const (
	// TimeFormatDateTimeMin 为精确到分钟的日期时间。
	TimeFormatDateTimeMin = "2006-01-02 15:04"
	// TimeFormatDateTimeNano 为带小数秒的日期时间。
	TimeFormatDateTimeNano = "2006-01-02 15:04:05.999999999"
	// TimeFormatDateTimeOffset 为带数值时区的日期时间。
	TimeFormatDateTimeOffset = "2006-01-02 15:04:05 -0700"
	// TimeFormatDateTimeRFC3339Offset 为带 RFC3339 时区的日期时间。
	TimeFormatDateTimeRFC3339Offset = "2006-01-02 15:04:05Z07:00"
	// TimeFormatDateTimeNanoOffset 为带小数秒和数值时区的日期时间。
	TimeFormatDateTimeNanoOffset = "2006-01-02 15:04:05.999999999 -0700"
	// TimeFormatDateTimeNanoRFC3339Offset 为带小数秒和 RFC3339 时区的日期时间。
	TimeFormatDateTimeNanoRFC3339Offset = "2006-01-02 15:04:05.999999999Z07:00"
	// TimeFormatDateSlash 为 2006/01/02。
	TimeFormatDateSlash = "2006/01/02"
	// TimeFormatDateTimeSlash 为 2006/01/02 15:04:05。
	TimeFormatDateTimeSlash = "2006/01/02 15:04:05"
	// TimeFormatDateTimeSlashMin 为精确到分钟的斜杠日期时间。
	TimeFormatDateTimeSlashMin = "2006/01/02 15:04"
	// TimeFormatDateTimeSlashNano 为带小数秒的斜杠日期时间。
	TimeFormatDateTimeSlashNano = "2006/01/02 15:04:05.999999999"
	// TimeFormatDateDot 为 2006.01.02。
	TimeFormatDateDot = "2006.01.02"
	// TimeFormatDateTimeDot 为 2006.01.02 15:04:05。
	TimeFormatDateTimeDot = "2006.01.02 15:04:05"
	// TimeFormatDateCompact 为 20060102。
	TimeFormatDateCompact = "20060102"
	// TimeFormatDateTimeCompact 为 20060102150405。
	TimeFormatDateTimeCompact = "20060102150405"
	// TimeFormatRFC3339NoTZ 为不带时区的 RFC3339。
	TimeFormatRFC3339NoTZ = "2006-01-02T15:04:05"
	// TimeFormatRFC3339NoTZNano 为不带时区、带小数秒的 RFC3339。
	TimeFormatRFC3339NoTZNano = "2006-01-02T15:04:05.999999999"
	// TimeFormatRFC3339UTC 为 UTC 的 RFC3339。
	TimeFormatRFC3339UTC = "2006-01-02T15:04:05Z"
	// TimeFormatDateChinese 为 2006年01月02日。
	TimeFormatDateChinese = "2006年01月02日"
	// TimeFormatDateTimeChinese 为 2006年01月02日 15:04:05。
	TimeFormatDateTimeChinese = "2006年01月02日 15:04:05"
	// TimeFormatDateChineseShort 为 2006年1月2日。
	TimeFormatDateChineseShort = "2006年1月2日"
	// TimeFormatDateTimeChineseShort 为 2006年1月2日 15:04:05。
	TimeFormatDateTimeChineseShort = "2006年1月2日 15:04:05"
	// TimeFormatUSDate 为美式 MM/DD/YYYY。
	TimeFormatUSDate = "01/02/2006"
	// TimeFormatUSDateTime 为美式日期时间。
	TimeFormatUSDateTime = "01/02/2006 15:04:05"
	// TimeFormatUSDateDash 为美式短横线日期。
	TimeFormatUSDateDash = "01-02-2006"
	// TimeFormatUSDateTimeDash 为美式短横线日期时间。
	TimeFormatUSDateTimeDash = "01-02-2006 15:04:05"
	// TimeFormatUSDateDot 为美式点分隔日期。
	TimeFormatUSDateDot = "01.02.2006"
	// TimeFormatUSDateTimeDot 为美式点分隔日期时间。
	TimeFormatUSDateTimeDot = "01.02.2006 15:04:05"
	// TimeFormatEuropeanDate 为欧式 DD/MM/YYYY。
	TimeFormatEuropeanDate = "02/01/2006"
	// TimeFormatEuropeanDateTime 为欧式日期时间。
	TimeFormatEuropeanDateTime = "02/01/2006 15:04:05"
	// TimeFormatEuropeanDateDash 为欧式短横线日期。
	TimeFormatEuropeanDateDash = "02-01-2006"
	// TimeFormatEuropeanDateTimeDash 为欧式短横线日期时间。
	TimeFormatEuropeanDateTimeDash = "02-01-2006 15:04:05"
	// TimeFormatEuropeanDateDot 为欧式点分隔日期。
	TimeFormatEuropeanDateDot = "02.01.2006"
	// TimeFormatEuropeanDateTimeDot 为欧式点分隔日期时间。
	TimeFormatEuropeanDateTimeDot = "02.01.2006 15:04:05"
)

// 年在前、英文月份的 RFC/ANSIC。更长/更具体的在前。不含无年的 Stamp/Kitchen/TimeOnly。
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.RFC1123Z,
	time.RFC1123,
	time.RFC822Z,
	time.RFC850,
	time.RFC822,
	time.UnixDate,
	time.ANSIC,
	time.RubyDate,
	TimeFormatRFC3339NoTZNano,
	TimeFormatRFC3339NoTZ,
	TimeFormatRFC3339UTC,
	TimeFormatDateTimeNanoOffset,
	TimeFormatDateTimeNanoRFC3339Offset,
	TimeFormatDateTimeNano,
	TimeFormatDateTimeOffset,
	TimeFormatDateTimeRFC3339Offset,
	time.DateTime,
	TimeFormatDateTimeMin,
	time.DateOnly,
	TimeFormatDateTimeSlashNano,
	TimeFormatDateTimeSlash,
	TimeFormatDateTimeSlashMin,
	TimeFormatDateSlash,
	TimeFormatDateTimeDot,
	TimeFormatDateDot,
	TimeFormatDateTimeChinese,
	TimeFormatDateChinese,
	TimeFormatDateTimeChineseShort,
	TimeFormatDateChineseShort,
	TimeFormatDateTimeCompact,
	TimeFormatDateCompact,
}

// 美式 MM/DD 与欧式 DD/MM 成对尝试，仅当只有一种能成立（或结果相同）时接受。
var usEuropeanLayouts = [][2]string{
	{TimeFormatUSDateTime, TimeFormatEuropeanDateTime},
	{TimeFormatUSDate, TimeFormatEuropeanDate},
	{TimeFormatUSDateTimeDash, TimeFormatEuropeanDateTimeDash},
	{TimeFormatUSDateDash, TimeFormatEuropeanDateDash},
	{TimeFormatUSDateTimeDot, TimeFormatEuropeanDateTimeDot},
	{TimeFormatUSDateDot, TimeFormatEuropeanDateDot},
}

// durationOrigin 故意早于进程启动，避免 DurationNow() 为 0。 因为程序会吧DurationNow认做还没有打点
var durationOrigin = time.Now().AddDate(-1, -1, -1)

// DurationNow 把「现在」编成 Duration，便于原子存储（int64）。
// 场景：记录「上次发生时刻」，0 表示从未发生（限频、熔断强制放行、日志丢弃）。
// 问题：零点若是启动瞬间，启动后立刻调用也是 0，「从未发生」和「刚发生」撞车，
// 限频每次都当成第一次，或第一条日志就被丢掉。所以零点提前约一年。
//
//	var last time.Duration // 0 = 从未发生
//	now := DurationNow()
//	if last == 0 || DurationSince(last) > interval {
//		last = now
//	}
func DurationNow() time.Duration {
	return time.Since(durationOrigin)
}

// DurationSince 返回从 start 到现在的间隔。start 必须来自 DurationNow。
func DurationSince(start time.Duration) time.Duration {
	return DurationNow() - start
}

// TimeFromUnix 将正数 Unix 秒转换为 time.Time，非正数返回零值。
func TimeFromUnix(seconds int64) time.Time {
	if seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

// TimeFromUnixMilli 将正数 Unix 毫秒转换为 time.Time，非正数返回零值。
func TimeFromUnixMilli(milliseconds int64) time.Time {
	if milliseconds <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(milliseconds)
}

// TimeToUnix 将 time.Time 转为 Unix 秒，零值返回 0。
func TimeToUnix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.Unix()
}

// TimeToUnixMilli 将 time.Time 转为 Unix 毫秒，零值返回 0。
func TimeToUnixMilli(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}

// TimeFromString 解析常见时间字符串。无时区时用 Local；空串或无法识别返回零值。
// 支持 RFC3339、RFC1123/822、ANSIC、年在前的日期时间、中文年月日、10/13 位 Unix。
// 美式/欧式数字日期仅在只有一种能成立时接受（15/01/2024 可以，03/04/2024 拒绝）。
func TimeFromString(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed := parseUnixTime(value); !parsed.IsZero() {
		return parsed
	}
	for _, layout := range timeLayouts {
		if parsed, ok := parseTimeLayout(layout, value); ok {
			return parsed
		}
	}
	for _, pair := range usEuropeanLayouts {
		usTime, usOK := parseTimeLayout(pair[0], value)
		europeanTime, europeanOK := parseTimeLayout(pair[1], value)
		switch {
		case usOK && europeanOK:
			if usTime.Equal(europeanTime) {
				return usTime
			}
		case usOK:
			return usTime
		case europeanOK:
			return europeanTime
		}
	}
	return time.Time{}
}

func parseTimeLayout(layout, value string) (time.Time, bool) {
	parsed, err := time.ParseInLocation(layout, value, time.Local)
	if err != nil || parsed.Year() < 1 {
		return time.Time{}, false
	}
	switch layout {
	case time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123,
		time.RFC822Z, time.RFC850, time.RFC822, time.UnixDate, time.ANSIC, time.RubyDate:
		return parsed, true
	}
	if parsed.Format(layout) != value {
		return time.Time{}, false
	}
	return parsed, true
}

func parseUnixTime(value string) time.Time {
	if len(value) != 10 && len(value) != 13 {
		return time.Time{}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return time.Time{}
	}
	if len(value) == 13 {
		return TimeFromUnixMilli(n)
	}
	return TimeFromUnix(n)
}

// DurationFromSeconds 将正数秒转换为 Duration，非正数返回 0。
func DurationFromSeconds(seconds int64) time.Duration {
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

// DurationToSeconds 将 Duration 转为秒，向下取整。
func DurationToSeconds(value time.Duration) int64 {
	return int64(value / time.Second)
}
