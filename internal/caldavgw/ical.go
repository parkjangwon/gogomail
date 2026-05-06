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
	MaxICalendarComponents          = 1024
	MaxICalendarProperties          = 8192
	MaxICalendarUIDBytes            = MaxCalendarObjectUIDBytes
	MaxICalendarRecurrenceInstances = 10000
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
	if err := validateICalendarRootSemantics(cal.Component); err != nil {
		return ICalendarObject{}, err
	}
	var found []calendarComponentObject
	for _, child := range cal.Children {
		component := strings.ToUpper(strings.TrimSpace(child.Name))
		switch component {
		case ComponentVEVENT, ComponentVTODO, ComponentVJOURNAL, ComponentVFREEBUSY:
			if err := validateICalendarComponentSemantics(component, child); err != nil {
				return ICalendarObject{}, err
			}
			uid, err := calendarComponentUID(child)
			if err != nil {
				return ICalendarObject{}, err
			}
			hasRecurrenceID := false
			if _, ok, err := eventRecurrenceID(child); err != nil {
				return ICalendarObject{}, err
			} else {
				hasRecurrenceID = ok
			}
			found = append(found, calendarComponentObject{
				ICalendarObject: ICalendarObject{UID: uid, Component: component},
				HasRecurrenceID: hasRecurrenceID,
			})
		}
	}
	if len(found) == 0 {
		return ICalendarObject{}, fmt.Errorf("iCalendar object must contain a supported calendar component")
	}
	if len(found) > 1 {
		if err := validateDetachedComponents(found); err != nil {
			return ICalendarObject{}, err
		}
	}
	return found[0].ICalendarObject, nil
}

func validateICalendarRootSemantics(root *ical.Component) error {
	if len(root.Props[ical.PropVersion]) != 1 {
		return fmt.Errorf("VCALENDAR must contain exactly one VERSION property")
	}
	version, err := root.Props[ical.PropVersion][0].Text()
	if err != nil {
		return fmt.Errorf("decode VCALENDAR VERSION: %w", err)
	}
	if strings.TrimSpace(version) != "2.0" {
		return fmt.Errorf("VCALENDAR VERSION must be 2.0")
	}
	if len(root.Props[ical.PropProductID]) != 1 {
		return fmt.Errorf("VCALENDAR must contain exactly one PRODID property")
	}
	productID, err := root.Props[ical.PropProductID][0].Text()
	if err != nil {
		return fmt.Errorf("decode VCALENDAR PRODID: %w", err)
	}
	if strings.TrimSpace(productID) == "" {
		return fmt.Errorf("VCALENDAR PRODID must not be empty")
	}
	if len(root.Props[ical.PropMethod]) > 0 {
		return fmt.Errorf("VCALENDAR calendar object resource must not contain METHOD")
	}
	return nil
}

type calendarComponentObject struct {
	ICalendarObject
	HasRecurrenceID bool
}

func validateDetachedComponents(found []calendarComponentObject) error {
	first := found[0]
	if first.Component != ComponentVEVENT {
		return fmt.Errorf("iCalendar object with multiple supported components must contain recurring VEVENT overrides")
	}
	masters := 0
	for _, object := range found {
		if object.UID != first.UID || object.Component != first.Component {
			return fmt.Errorf("iCalendar object with multiple supported components must use one UID and component type")
		}
		if object.HasRecurrenceID {
			continue
		}
		masters++
	}
	if masters != 1 {
		return fmt.Errorf("iCalendar recurring VEVENT object must contain exactly one master component")
	}
	return nil
}

func validateICalendarComponentSemantics(component string, child *ical.Component) error {
	switch component {
	case ComponentVEVENT:
		if err := validateICalendarSingletonProps(component, child, []string{
			ical.PropDateTimeStamp,
			ical.PropDateTimeStart,
			ical.PropDateTimeEnd,
			ical.PropDuration,
			ical.PropStatus,
			ical.PropTransparency,
			ical.PropRecurrenceID,
		}); err != nil {
			return err
		}
		if len(child.Props[ical.PropDateTimeEnd]) > 0 && len(child.Props[ical.PropDuration]) > 0 {
			return fmt.Errorf("VEVENT must not contain both DTEND and DURATION")
		}
	case ComponentVTODO:
		if err := validateICalendarSingletonProps(component, child, []string{
			ical.PropDateTimeStamp,
			ical.PropDateTimeStart,
			ical.PropDue,
			ical.PropDuration,
			ical.PropStatus,
		}); err != nil {
			return err
		}
		if len(child.Props[ical.PropDue]) > 0 && len(child.Props[ical.PropDuration]) > 0 {
			return fmt.Errorf("VTODO must not contain both DUE and DURATION")
		}
		if len(child.Props[ical.PropDuration]) > 0 && len(child.Props[ical.PropDateTimeStart]) == 0 {
			return fmt.Errorf("VTODO with DURATION must contain DTSTART")
		}
	case ComponentVJOURNAL:
		if err := validateICalendarSingletonProps(component, child, []string{
			ical.PropDateTimeStamp,
			ical.PropDateTimeStart,
			ical.PropStatus,
		}); err != nil {
			return err
		}
	case ComponentVFREEBUSY:
		if err := validateICalendarSingletonProps(component, child, []string{
			ical.PropDateTimeStamp,
			ical.PropDateTimeStart,
			ical.PropDateTimeEnd,
		}); err != nil {
			return err
		}
	}
	return nil
}

func validateICalendarSingletonProps(component string, child *ical.Component, names []string) error {
	for _, name := range names {
		if len(child.Props[name]) > 1 {
			return fmt.Errorf("%s must not contain multiple %s properties", component, name)
		}
	}
	return nil
}

func ProjectCalendarData(body []byte, req CalendarDataRequest) ([]byte, error) {
	if !req.Requested || !req.HasProjection {
		return append([]byte(nil), body...), nil
	}
	cal, err := ical.NewDecoder(bytes.NewReader(body)).Decode()
	if err != nil {
		return nil, fmt.Errorf("decode iCalendar object: %w", err)
	}
	if cal == nil || cal.Component == nil || strings.ToUpper(cal.Name) != ical.CompCalendar {
		return nil, fmt.Errorf("iCalendar body must contain one VCALENDAR root")
	}
	projected := &ical.Calendar{Component: ical.NewComponent(ical.CompCalendar)}
	projected.Props = projectICalendarProps(cal.Props, req.CalendarProperties, "VCALENDAR")
	for _, child := range cal.Children {
		component := strings.ToUpper(strings.TrimSpace(child.Name))
		if req.Component != "" && !strings.EqualFold(component, req.Component) {
			continue
		}
		projected.Children = append(projected.Children, &ical.Component{
			Name:     child.Name,
			Props:    projectICalendarProps(child.Props, req.ComponentProperties, component),
			Children: projectICalendarChildren(child.Children),
		})
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(projected); err != nil {
		return nil, fmt.Errorf("encode projected iCalendar object: %w", err)
	}
	return buf.Bytes(), nil
}

func projectICalendarProps(props ical.Props, selected map[string]bool, component string) ical.Props {
	projected := make(ical.Props)
	required := requiredProjectionProperties(component)
	for name, values := range props {
		upperName := strings.ToUpper(name)
		if len(selected) > 0 && !selected[upperName] && !required[upperName] {
			continue
		}
		projected[name] = append([]ical.Prop(nil), values...)
	}
	return projected
}

func requiredProjectionProperties(component string) map[string]bool {
	switch strings.ToUpper(strings.TrimSpace(component)) {
	case "VCALENDAR":
		return map[string]bool{"VERSION": true, "PRODID": true}
	case ComponentVEVENT, ComponentVTODO, ComponentVJOURNAL, ComponentVFREEBUSY:
		return map[string]bool{"UID": true, "DTSTAMP": true}
	default:
		return nil
	}
}

func projectICalendarChildren(children []*ical.Component) []*ical.Component {
	projected := make([]*ical.Component, 0, len(children))
	for _, child := range children {
		projected = append(projected, &ical.Component{
			Name:     child.Name,
			Props:    projectICalendarProps(child.Props, nil, child.Name),
			Children: projectICalendarChildren(child.Children),
		})
	}
	return projected
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
	excludedRecurrences := veventOverrideRecurrenceIDs(cal.Children)
	for _, child := range cal.Children {
		if strings.EqualFold(child.Name, ComponentVEVENT) {
			uid, err := calendarComponentUID(child)
			if err != nil {
				return false, err
			}
			excluded := excludedRecurrences[uid]
			if child.Props.Get(ical.PropRecurrenceID) != nil {
				excluded = nil
			}
			matches, err := eventOverlapsRange(child, *timeRange, excluded)
			if err != nil {
				return false, err
			}
			if matches {
				return true, nil
			}
		}
	}
	return false, nil
}

func eventOverlapsRange(component *ical.Component, timeRange TimeRange, excludedRecurrences map[int64]struct{}) (bool, error) {
	start, end, duration, err := eventTimeSpanWithDuration(component)
	if err != nil {
		return false, err
	}
	recurrence, err := component.RecurrenceSet(time.UTC)
	if err != nil {
		return false, fmt.Errorf("decode VEVENT recurrence: %w", err)
	}
	if recurrence == nil {
		explicitStarts, explicit, err := explicitRecurrenceStarts(component, start, excludedRecurrences)
		if err != nil {
			return false, err
		}
		if explicit {
			return explicitRecurrencesOverlap(explicitStarts, duration, timeRange), nil
		}
		return timeSpansOverlap(start, end, timeRange), nil
	}
	windowStart := recurrenceWindowStart(timeRange.Start, duration)
	next := recurrence.Iterator()
	scanned := 0
	for occurrenceStart, ok := next(); ok; occurrenceStart, ok = next() {
		scanned++
		if scanned > MaxICalendarRecurrenceInstances {
			return false, fmt.Errorf("VEVENT recurrence expansion exceeds %d instances", MaxICalendarRecurrenceInstances)
		}
		occurrenceStart = occurrenceStart.UTC()
		if occurrenceStart.Before(windowStart) {
			continue
		}
		if !occurrenceStart.Before(timeRange.End) {
			return false, nil
		}
		if recurrenceExcluded(occurrenceStart, excludedRecurrences) {
			continue
		}
		if timeSpansOverlap(occurrenceStart, occurrenceStart.Add(duration), timeRange) {
			return true, nil
		}
	}
	return false, nil
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
	excludedRecurrences := veventOverrideRecurrenceIDs(cal.Children)
	for _, child := range cal.Children {
		switch {
		case strings.EqualFold(child.Name, ComponentVEVENT):
			uid, err := calendarComponentUID(child)
			if err != nil {
				return nil, err
			}
			excluded := excludedRecurrences[uid]
			if child.Props.Get(ical.PropRecurrenceID) != nil {
				excluded = nil
			}
			objectPeriods, err := eventBusyPeriods(child, timeRange, excluded)
			if err != nil {
				return nil, err
			}
			periods = append(periods, objectPeriods...)
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

func eventBusyPeriods(component *ical.Component, timeRange TimeRange, excludedRecurrences map[int64]struct{}) ([]BusyPeriod, error) {
	event := ical.Event{Component: component}
	status, err := event.Status()
	if err != nil {
		return nil, fmt.Errorf("decode VEVENT STATUS: %w", err)
	}
	if strings.EqualFold(string(status), string(ical.EventCancelled)) {
		return nil, nil
	}
	transparency, err := component.Props.Text(ical.PropTransparency)
	if err != nil {
		return nil, fmt.Errorf("decode VEVENT TRANSP: %w", err)
	}
	if strings.EqualFold(transparency, "TRANSPARENT") {
		return nil, nil
	}
	start, end, duration, err := eventTimeSpanWithDuration(component)
	if err != nil {
		return nil, err
	}
	busyType := "BUSY"
	if strings.EqualFold(string(status), string(ical.EventTentative)) {
		busyType = "BUSY-TENTATIVE"
	}
	recurrence, err := component.RecurrenceSet(time.UTC)
	if err != nil {
		return nil, fmt.Errorf("decode VEVENT recurrence: %w", err)
	}
	if recurrence == nil {
		explicitStarts, explicit, err := explicitRecurrenceStarts(component, start, excludedRecurrences)
		if err != nil {
			return nil, err
		}
		if explicit {
			return explicitRecurrenceBusyPeriods(explicitStarts, duration, busyType, timeRange), nil
		}
		period, ok := clippedBusyPeriod(start, end, busyType, timeRange)
		if !ok {
			return nil, nil
		}
		return []BusyPeriod{period}, nil
	}
	windowStart := recurrenceWindowStart(timeRange.Start, duration)
	next := recurrence.Iterator()
	periods := make([]BusyPeriod, 0)
	scanned := 0
	for occurrenceStart, ok := next(); ok; occurrenceStart, ok = next() {
		scanned++
		if scanned > MaxICalendarRecurrenceInstances {
			return nil, fmt.Errorf("VEVENT recurrence expansion exceeds %d instances", MaxICalendarRecurrenceInstances)
		}
		occurrenceStart = occurrenceStart.UTC()
		if occurrenceStart.Before(windowStart) {
			continue
		}
		if !occurrenceStart.Before(timeRange.End) {
			return periods, nil
		}
		if recurrenceExcluded(occurrenceStart, excludedRecurrences) {
			continue
		}
		period, ok := clippedBusyPeriod(occurrenceStart, occurrenceStart.Add(duration), busyType, timeRange)
		if ok {
			periods = append(periods, period)
		}
	}
	return periods, nil
}

func veventOverrideRecurrenceIDs(children []*ical.Component) map[string]map[int64]struct{} {
	excluded := make(map[string]map[int64]struct{})
	for _, child := range children {
		if !strings.EqualFold(child.Name, ComponentVEVENT) {
			continue
		}
		recurrenceID, ok, err := eventRecurrenceID(child)
		if err != nil || !ok {
			continue
		}
		uid, err := calendarComponentUID(child)
		if err != nil {
			continue
		}
		bucket := excluded[uid]
		if bucket == nil {
			bucket = make(map[int64]struct{})
			excluded[uid] = bucket
		}
		bucket[recurrenceID.UnixNano()] = struct{}{}
	}
	return excluded
}

func eventRecurrenceID(component *ical.Component) (time.Time, bool, error) {
	prop := component.Props.Get(ical.PropRecurrenceID)
	if prop == nil {
		return time.Time{}, false, nil
	}
	date, err := prop.DateTime(time.UTC)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("decode VEVENT RECURRENCE-ID: %w", err)
	}
	return date.UTC(), true, nil
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

func eventTimeSpanWithDuration(component *ical.Component) (time.Time, time.Time, time.Duration, error) {
	start, end, err := eventTimeSpan(component)
	if err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	duration := end.Sub(start)
	if duration <= 0 {
		duration = time.Nanosecond
		end = start.Add(duration)
	}
	return start, end, duration, nil
}

func explicitRecurrenceStarts(component *ical.Component, start time.Time, excludedRecurrences map[int64]struct{}) ([]time.Time, bool, error) {
	explicit := len(component.Props[ical.PropRecurrenceDates]) > 0 || len(component.Props[ical.PropExceptionDates]) > 0
	starts := []time.Time{start.UTC()}
	for _, prop := range component.Props[ical.PropRecurrenceDates] {
		date, err := prop.DateTime(time.UTC)
		if err != nil {
			return nil, explicit, fmt.Errorf("decode VEVENT RDATE: %w", err)
		}
		starts = append(starts, date.UTC())
	}
	excluded := make(map[int64]struct{}, len(component.Props[ical.PropExceptionDates]))
	for _, prop := range component.Props[ical.PropExceptionDates] {
		date, err := prop.DateTime(time.UTC)
		if err != nil {
			return nil, explicit, fmt.Errorf("decode VEVENT EXDATE: %w", err)
		}
		excluded[date.UTC().UnixNano()] = struct{}{}
	}
	filtered := starts[:0]
	for _, occurrenceStart := range starts {
		if _, ok := excluded[occurrenceStart.UnixNano()]; ok {
			continue
		}
		if recurrenceExcluded(occurrenceStart, excludedRecurrences) {
			continue
		}
		filtered = append(filtered, occurrenceStart)
	}
	return filtered, explicit, nil
}

func recurrenceExcluded(occurrenceStart time.Time, excludedRecurrences map[int64]struct{}) bool {
	if len(excludedRecurrences) == 0 {
		return false
	}
	_, ok := excludedRecurrences[occurrenceStart.UTC().UnixNano()]
	return ok
}

func explicitRecurrencesOverlap(starts []time.Time, duration time.Duration, timeRange TimeRange) bool {
	for i, occurrenceStart := range starts {
		if i >= MaxICalendarRecurrenceInstances {
			return false
		}
		if timeSpansOverlap(occurrenceStart, occurrenceStart.Add(duration), timeRange) {
			return true
		}
	}
	return false
}

func explicitRecurrenceBusyPeriods(starts []time.Time, duration time.Duration, busyType string, timeRange TimeRange) []BusyPeriod {
	periods := make([]BusyPeriod, 0, len(starts))
	for i, occurrenceStart := range starts {
		if i >= MaxICalendarRecurrenceInstances {
			return periods
		}
		period, ok := clippedBusyPeriod(occurrenceStart, occurrenceStart.Add(duration), busyType, timeRange)
		if ok {
			periods = append(periods, period)
		}
	}
	return periods
}

func recurrenceWindowStart(rangeStart time.Time, duration time.Duration) time.Time {
	windowStart := rangeStart.Add(-duration)
	if duration < time.Second {
		windowStart = windowStart.Add(-time.Second)
	}
	return windowStart.UTC()
}

func timeSpansOverlap(start time.Time, end time.Time, timeRange TimeRange) bool {
	return start.Before(timeRange.End) && end.After(timeRange.Start)
}

func clippedBusyPeriod(start time.Time, end time.Time, busyType string, timeRange TimeRange) (BusyPeriod, bool) {
	if !timeSpansOverlap(start, end, timeRange) {
		return BusyPeriod{}, false
	}
	if start.Before(timeRange.Start) {
		start = timeRange.Start
	}
	if end.After(timeRange.End) {
		end = timeRange.End
	}
	return BusyPeriod{Start: start.UTC(), End: end.UTC(), Type: busyType}, true
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
