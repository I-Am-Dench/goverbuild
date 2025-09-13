// Rudimentary charxml implementation
//
// TODO:
//   - Flags
//   - Active missions
//   - Inventory
//   - Zone stats
//   - Pets
//   - Rocket parts (lrid, lcbp)
//   - Point struct instead of including individual fields
//   - Marshalling
//   - Add and move structs into a components package?
package charxml

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

type Stats struct {
	CurrencyCollected                  uint64
	BricksCollected                    int64
	SmashablesSmashed                  uint64
	QuickBuildsCompleted               uint64
	EnemiesSmashed                     uint64
	RocketsUsed                        uint64
	MissionsCompleted                  uint64
	PetsTamed                          uint64
	ImaginationPowerUpsCollected       uint64
	LifePowerUpsCollected              uint64
	ArmorPowerUpsCollected             uint64
	MetersTraveled                     uint64
	TimesSmashed                       uint64
	TotalDamageTaken                   uint64
	TotalDamageHealed                  uint64
	TotalArmorRepaired                 uint64
	TotalImaginationRestored           uint64
	TotalImaginationUsed               uint64
	DistanceDriven                     uint64
	TimeAirborneInCar                  uint64
	RacingImaginationPowerUpsCollected uint64
	RacingImaginationCratesSmashed     uint64
	RacingCarBootsActivated            uint64
	RacingTimesWrecked                 uint64
	RacingSmashablesSmashed            uint64
	RacesFinished                      uint64
	FirstPlaceRaceFinishes             uint64
}

// Need to benchmark using reflection vs. hardcoding parsing
func (stats *Stats) UnmarshalXMLAttr(attr xml.Attr) error {
	parts := strings.Split(attr.Value, ";")

	structValue := reflect.ValueOf(stats).Elem()
	structType := structValue.Type()

	numFields := min(structType.NumField(), len(parts))
	for i := 0; i < numFields; i++ {
		field := structValue.Field(i)

		switch field.Kind() {
		case reflect.Int64:
			i, err := strconv.ParseInt(parts[i], 10, 64)
			if err == nil {
				field.SetInt(i)
			}
		case reflect.Uint64:
			i, err := strconv.ParseUint(parts[i], 10, 64)
			if err == nil {
				field.SetUint(i)
			}
		}
	}

	return nil
}

type Emote int32

func (emote *Emote) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	if start.Name.Local != "e" {
		return fmt.Errorf("unexpected tag: %s", start.Name.Local)
	}

	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "id":
			i, err := strconv.Atoi(attr.Value)
			if err == nil {
				*emote = Emote(i)
			}
		}
	}

	return d.Skip()
}

type Emotes []Emote

func (emotes *Emotes) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	collected := []Emote{}

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
			var emote Emote
			err := d.DecodeElement(&emote, &token)
			if errors.Is(err, io.EOF) {
				break loop
			}

			if err != nil {
				return err
			}

			collected = append(collected, emote)
		case xml.EndElement:
			if token.Name.Local == start.Name.Local {
				break
			}
		}
	}

	*emotes = collected

	return nil
}

type Character struct {
	AccountId        uint64  `xml:"acct,attr"`
	Currency         uint64  `xml:"cc,attr"`
	MaxCurrency      uint64  `xml:"cm,attr"`
	ClaimCode        uint64  `xml:"co,attr"`
	FreeToPlay       bool    `xml:"ft,attr"`
	GMLevel          GMLevel `xml:"gm,attr"`
	LastLogin        int64   `xml:"llog,attr"`
	LastRespawnPosX  float32 `xml:"lrx,attr"`
	LastRespawnPosY  float32 `xml:"lry,attr"`
	LastRespawnPosZ  float32 `xml:"lrz,attr"`
	LastRespawnRotW  float32 `xml:"lrrw,attr"`
	LastRespawnRotX  float32 `xml:"lrrx,attr"`
	LastRespawnRotY  float32 `xml:"lrry,attr"`
	LastRespawnRotZ  float32 `xml:"lrrz,attr"`
	UniverseScore    uint64  `xml:"ls,attr"`
	LastZoneChecksum uint32  `xml:"lzcs,attr"`
	LastZoneId       uint64  `xml:"lzid,attr"`
	LastZoneRotW     float32 `xml:"lzrw,attr"`
	LastZoneRotX     float32 `xml:"lzrx,attr"`
	LastZoneRotY     float32 `xml:"lzry,attr"`
	LastZoneRotZ     float32 `xml:"lzrz,attr"`
	LastZonePosX     float32 `xml:"lzx,attr"`
	LastZonePosY     float32 `xml:"lzy,attr"`
	LastZonePosZ     float32 `xml:"lzz,attr"`
	LastPropModTime  int64   `xml:"mldt,attr"`
	LastWorldId      uint32  `xml:"lwid,attr"`
	Stats            *Stats  `xml:"stt,attr"`
	TotalPlayTime    uint64  `xml:"time,attr"`
	Reputation       int64   `xml:"rpt,attr"`
	Emotes           Emotes  `xml:"ue"`
}

type Level struct {
	Level     uint32 `xml:"l,attr"`
	SpeedBase uint32 `xml:"sb,attr"`
}

type Obj struct {
	XMLName   xml.Name  `xml:"obj"`
	Character Character `xml:"char"`
	Level     Level     `xml:"lvl"`
	Missions  struct {
		Done CompletedMissions `xml:"done"`
	} `xml:"mis"`
}
