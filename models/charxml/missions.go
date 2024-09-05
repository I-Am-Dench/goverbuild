package charxml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

type MissionState int

// Taken from https://github.com/DarkflameUniverse/DarkflameServer/blob/main/dCommon/dEnums/eMissionState.h
const (
	MissionUnknown              = MissionState(-1)
	MissionRewarding            = MissionState(0)
	MissionAvailable            = MissionState(1)
	MissionActive               = MissionState(2)
	MissionReadyToComplete      = MissionState(4)
	MissionComplete             = MissionState(8)
	MissionAvailableAgain       = MissionState(9)
	MissionActiveAgain          = MissionState(10)
	MissionReadyToCompleteAgain = MissionState(12)
)

type CompletedMission struct {
	Id             uint32
	TimesCompleted uint32
	CompletionTime time.Time
}

func (mission *CompletedMission) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "id":
			i, err := strconv.Atoi(attr.Value)
			if err == nil {
				mission.Id = uint32(i)
			}
		case "cct":
			i, err := strconv.Atoi(attr.Value)
			if err == nil {
				mission.TimesCompleted = uint32(i)
			}
		case "cts":
			i, err := strconv.ParseInt(attr.Value, 10, 64)
			if err == nil {
				mission.CompletionTime = time.Unix(i, 0)
			}
		}
	}

	return d.Skip()
}

type CompletedMissions map[uint32]CompletedMission

func (missions *CompletedMissions) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	completed := CompletedMissions{}

loop:
	for {
		t, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		switch token := t.(type) {
		case xml.StartElement:
			if token.Name.Local != "m" {
				return fmt.Errorf("unexpected tag: %s", start.Name.Local)
			}

			mission := CompletedMission{}
			err := d.Decode(&mission)
			if errors.Is(err, io.EOF) {
				break loop
			}

			if err != nil {
				return nil
			}

			completed[mission.Id] = mission
		case xml.EndElement:
			if token.Name.Local == start.Name.Local {
				break loop
			}
		}
	}

	*missions = completed

	return nil
}
