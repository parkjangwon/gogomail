package caldavgw

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"
)

const (
	MaxICalendarComponents = 1024
	MaxICalendarProperties = 8192
	MaxICalendarUIDBytes   = MaxCalendarObjectUIDBytes
)

type ICalendarObject struct {
	UID       string
	Component string
}

type BusyPeriod struct {
	Start time.Time
	End   time.Time
	Type  string
}

func ParseICalendarObject(body []byte) (ICalendarObject, error) {
	if _, err := StrongETag(body); err != nil {
		return ICalendarObject{}, err
	}
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return ICalendarObject{}, fmt.Errorf("decode iCalendar object: %w", err)
	}
	if cal == nil || cal.Component == nil || strings.ToUpper(cal.Name) != ical.CompCalendar {
		return ICalendarObject{}, fmt.Errorf("iCalendar body must contain one VCALENDAR root")
	}
	if err := validateICalendarBounds(cal.Component); err != nil {
		return ICalendarObject{}, err
	}
	var found []ICalendarObject
	for _, child := range cal.Children {
		component := strings.ToUpper(strings.TrimSpace(child.Name))
		switch component {
		case ComponentVEVENT, ComponentVTODO, ComponentVJOURNAL, ComponentVFREEBUSY:
			uid, err := calendarComponentUID(child)
			if err != nil {
				return ICalendarObject{}, err
			}
			found = append(found, ICalendarObject{UID: uid, Component: component})
		}
	}
	if len(found) == 0 {
		return ICalendarObject{}, fmt.Errorf("iCalendar object must contain a supported calendar component")
	}
	if len(found) > 1 {
		return ICalendarObject{}, fmt.Errorf("iCalendar object must contain exactly one supported calendar component")
	}
	return found[0], nil
}

func CalendarObjectMatchesTimeRange(body []byte, timeRange *TimeRange) (bool, error) {
	if timeRange == nil {
		return true, nil
	}
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return false, fmt.Errorf("decode iCalendar object: %w", err)
	}
	if cal == nil || cal.Component == nil {
		return false, fmt.Errorf("iCalendar body must contain one VCALENDAR root")
	}
	for _, child := range cal.Children {
		if strings.EqualFold(child.Name, ComponentVEVENT) {
			return eventOverlapsRange(child, *timeRange)
		}
	}
	return false, nil
}

func eventOverlapsRange(component *ical.Component, timeRange TimeRange) (bool, error) {
	start, end, err := eventTimeSpan(component)
	if err != nil {
		return false, err
	}
	if end.Equal(start) {
		end = start.Add(time.Nanosecond)
	}
	return start.Before(timeRange.End) && end.After(timeRange.Start), nil
}

func CalendarObjectBusyPeriods(body []byte, timeRange TimeRange) ([]BusyPeriod, error) {
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return nil, fmt.Errorf("decode iCalendar object: %w", err)
	}
	if cal == nil || cal.Component == nil {
		return nil, fmt.Errorf("iCalendar body must contain one VCALENDAR root")
	}
	var periods []BusyPeriod
	for _, child := range cal.Children {
		switch {
		case strings.EqualFold(child.Name, ComponentVEVENT):
			period, ok, err := eventBusyPeriod(child, timeRange)
			if err != nil {
				return nil, err
			}
			if ok {
				periods = append(periods, period)
			}
		case strings.EqualFold(child.Name, ComponentVFREEBUSY):
			freeBusyPeriods, err := freeBusyComponentPeriods(child, timeRange)
			if err != nil {
				return nil, err
			}
			periods = append(periods, freeBusyPeriods...)
		}
	}
	return periods, nil
}

func eventBusyPeriod(component *ical.Component, timeRange TimeRange) (BusyPeriod, bool, error) {
	event := ical.Event{Component: component}
	status, err := event.Status()
	if err != nil {
		return BusyPeriod{}, false, fmt.Errorf("decode VEVENT STATUS: %w", err)
	}
	if strings.EqualFold(string(status), string(ical.EventCancelled)) {
		return BusyPeriod{}, false, nil
	}
	transparency, err := component.Props.Text(ical.PropTransparency)
	if err != nil {
		return BusyPeriod{}, false, fmt.Errorf("decode VEVENT TRANSP: %w", err)
	}
	if strings.EqualFold(transparency, "TRANSPARENT") {
		return BusyPeriod{}, false, nil
	}
	start, end, err := eventTimeSpan(component)
	if err != nil {
		return BusyPeriod{}, false, err
	}
	if !start.Before(end) || !start.Before(timeRange.End) || !end.After(timeRange.Start) {
		return BusyPeriod{}, false, nil
	}
	if start.Before(timeRange.Start) {
		start = timeRange.Start
	}
	if end.After(timeRange.End) {
		end = timeRange.End
	}
	busyType := "BUSY"
	if strings.EqualFold(string(status), string(ical.EventTentative)) {
		busyType = "BUSY-TENTATIVE"
	}
	return BusyPeriod{Start: start.UTC(), End: end.UTC(), Type: busyType}, true, nil
}

func freeBusyComponentPeriods(component *ical.Component, timeRange TimeRange) ([]BusyPeriod, error) {
	var periods []BusyPeriod
	for _, prop := range component.Props.Values(ical.PropFreeBusy) {
		busyType := "BUSY"
		if values := prop.Params.Get(ical.ParamFreeBusyType); values != "" {
			busyType = values
		}
		for _, rawPeriod := range strings.Split(prop.Value, ",") {
			period, ok, err := parseFreeBusyPeriod(rawPeriod, busyType, timeRange)
			if err != nil {
				return nil, err
			}
			if ok {
				periods = append(periods, period)
			}
		}
	}
	return periods, nil
}

func parseFreeBusyPeriod(value string, busyType string, timeRange TimeRange) (BusyPeriod, bool, error) {
	startText, endText, ok := strings.Cut(strings.TrimSpace(value), "/")
	if !ok {
		return BusyPeriod{}, false, fmt.Errorf("FREEBUSY period must contain start/end or start/duration")
	}
	start, err := parseICalendarUTC(strings.TrimSpace(startText))
	if err != nil {
		return BusyPeriod{}, false, fmt.Errorf("decode FREEBUSY period start: %w", err)
	}
	endValue := strings.TrimSpace(endText)
	var end time.Time
	if strings.HasPrefix(endValue, "P") || strings.HasPrefix(endValue, "+P") {
		duration, err := parseICalendarDuration(endValue)
		if err != nil {
			return BusyPeriod{}, false, fmt.Errorf("decode FREEBUSY period duration: %w", err)
		}
		end = start.Add(duration)
	} else {
		end, err = parseICalendarUTC(endValue)
		if err != nil {
			return BusyPeriod{}, false, fmt.Errorf("decode FREEBUSY period end: %w", err)
		}
	}
	if !start.Before(end) || !start.Before(timeRange.End) || !end.After(timeRange.Start) {
		return BusyPeriod{}, false, nil
	}
	if start.Before(timeRange.Start) {
		start = timeRange.Start
	}
	if end.After(timeRange.End) {
		end = timeRange.End
	}
	return BusyPeriod{Start: start.UTC(), End: end.UTC(), Type: busyType}, true, nil
}

func parseICalendarDuration(value string) (time.Duration, error) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "+")
	if value == "" || value[0] != 'P' {
		return 0, fmt.Errorf("duration must start with P")
	}
	var total time.Duration
	var number int64
	inTime := false
	hasPart := false
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if ch >= '0' && ch <= '9' {
			number = number*10 + int64(ch-'0')
			continue
		}
		if ch == 'T' && !inTime {
			inTime = true
			number = 0
			continue
		}
		if number <= 0 {
			return 0, fmt.Errorf("duration value is invalid")
		}
		switch ch {
		case 'W':
			if inTime || hasPart || i != len(value)-1 {
				return 0, fmt.Errorf("weeks must be the only duration field")
			}
			total += time.Duration(number) * 7 * 24 * time.Hour
		case 'D':
			if inTime {
				return 0, fmt.Errorf("days must precede time fields")
			}
			total += time.Duration(number) * 24 * time.Hour
		case 'H':
			if !inTime {
				return 0, fmt.Errorf("hours require time field")
			}
			total += time.Duration(number) * time.Hour
		case 'M':
			if !inTime {
				return 0, fmt.Errorf("months are not supported in durations")
			}
			total += time.Duration(number) * time.Minute
		case 'S':
			if !inTime {
				return 0, fmt.Errorf("seconds require time field")
			}
			total += time.Duration(number) * time.Second
		default:
			return 0, fmt.Errorf("unsupported duration field %q", ch)
		}
		number = 0
		hasPart = true
	}
	if number != 0 || !hasPart || total <= 0 {
		return 0, fmt.Errorf("duration value is invalid")
	}
	return total, nil
}

func eventTimeSpan(component *ical.Component) (time.Time, time.Time, error) {
	event := ical.Event{Component: component}
	start, err := event.DateTimeStart(time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("decode VEVENT DTSTART: %w", err)
	}
	end, err := event.DateTimeEnd(time.UTC)
	if err != nil || end.IsZero() {
		end = start
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("VEVENT DTEND must not be before DTSTART")
	}
	return start.UTC(), end.UTC(), nil
}

func BuildFreeBusyCalendar(userID string, calendarID string, timeRange TimeRange, periods []BusyPeriod) ([]byte, error) {
	userID = strings.TrimSpace(userID)
	calendarID = strings.TrimSpace(calendarID)
	if userID == "" || calendarID == "" {
		return nil, fmt.Errorf("free-busy user and calendar identifiers are required")
	}
	if !timeRange.Start.Before(timeRange.End) {
		return nil, fmt.Errorf("free-busy time range is invalid")
	}
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//gogomail//CalDAV FreeBusy//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropMethod, "REPLY")
	freeBusy := ical.NewComponent(ical.CompFreeBusy)
	freeBusy.Props.SetText(ical.PropUID, CalendarSyncToken(userID, calendarID, timeRange.Start.Format(time.RFC3339Nano), timeRange.End.Format(time.RFC3339Nano)))
	freeBusy.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	freeBusy.Props.SetDateTime(ical.PropDateTimeStart, timeRange.Start.UTC())
	freeBusy.Props.SetDateTime(ical.PropDateTimeEnd, timeRange.End.UTC())
	for _, period := range CoalesceBusyPeriods(periods) {
		if !period.Start.Before(period.End) {
			continue
		}
		prop := ical.NewProp(ical.PropFreeBusy)
		busyType := strings.TrimSpace(strings.ToUpper(period.Type))
		if busyType == "" {
			busyType = "BUSY"
		}
		prop.Params[ical.ParamFreeBusyType] = []string{busyType}
		prop.Value = formatICalendarUTC(period.Start) + "/" + formatICalendarUTC(period.End)
		freeBusy.Props.Add(prop)
	}
	cal.Children = append(cal.Children, freeBusy)
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		return nil, fmt.Errorf("encode free-busy calendar: %w", err)
	}
	return buf.Bytes(), nil
}

func CoalesceBusyPeriods(periods []BusyPeriod) []BusyPeriod {
	if len(periods) == 0 {
		return nil
	}
	normalized := make([]BusyPeriod, 0, len(periods))
	for _, period := range periods {
		if !period.Start.Before(period.End) {
			continue
		}
		period.Start = period.Start.UTC()
		period.End = period.End.UTC()
		period.Type = strings.TrimSpace(strings.ToUpper(period.Type))
		if period.Type == "" {
			period.Type = "BUSY"
		}
		normalized = append(normalized, period)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if !normalized[i].Start.Equal(normalized[j].Start) {
			return normalized[i].Start.Before(normalized[j].Start)
		}
		if normalized[i].Type != normalized[j].Type {
			return normalized[i].Type < normalized[j].Type
		}
		return normalized[i].End.Before(normalized[j].End)
	})
	coalesced := normalized[:0]
	for _, period := range normalized {
		if len(coalesced) == 0 {
			coalesced = append(coalesced, period)
			continue
		}
		last := &coalesced[len(coalesced)-1]
		if last.Type == period.Type && !period.Start.After(last.End) {
			if period.End.After(last.End) {
				last.End = period.End
			}
			continue
		}
		coalesced = append(coalesced, period)
	}
	return append([]BusyPeriod(nil), coalesced...)
}

func formatICalendarUTC(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func calendarComponentUID(component *ical.Component) (string, error) {
	if component == nil {
		return "", fmt.Errorf("calendar component is required")
	}
	values := component.Props.Values(ical.PropUID)
	if len(values) != 1 {
		return "", fmt.Errorf("calendar component must contain exactly one UID")
	}
	uid, err := values[0].Text()
	if err != nil {
		return "", fmt.Errorf("decode calendar component UID: %w", err)
	}
	return ValidateCalendarObjectUID(uid)
}

func validateICalendarBounds(root *ical.Component) error {
	components := 0
	properties := 0
	var walk func(component *ical.Component) error
	walk = func(component *ical.Component) error {
		if component == nil {
			return nil
		}
		components++
		if components > MaxICalendarComponents {
			return fmt.Errorf("iCalendar component count exceeds %d", MaxICalendarComponents)
		}
		for name, values := range component.Props {
			properties += len(values)
			if properties > MaxICalendarProperties {
				return fmt.Errorf("iCalendar property count exceeds %d", MaxICalendarProperties)
			}
			if strings.EqualFold(name, ical.PropUID) {
				for _, prop := range values {
					if len(prop.Value) > MaxICalendarUIDBytes {
						return fmt.Errorf("calendar component UID is too long")
					}
				}
			}
		}
		for _, child := range component.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root)
}
