package caldavgw

import (
	"bytes"
	"fmt"
	"strings"

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
