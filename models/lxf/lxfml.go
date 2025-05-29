package lxf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
)

type JointType string

const (
	JointTypeBall  = JointType("ball")
	JointTypeHinge = JointType("hinge")
)

type TransformationMatrix [4][3]float64

func (m TransformationMatrix) MarshalText() ([]byte, error) {
	buf := bytes.Buffer{}
	for i, row := range m {
		for j, n := range row {
			if !(i == 0 && j == 0) {
				buf.WriteRune(',')
			}
			buf.WriteString(strconv.FormatFloat(n, 'f', -1, 64))
		}
	}
	return buf.Bytes(), nil
}

func (m *TransformationMatrix) UnmarshalText(text []byte) error {
	parts := bytes.Split(text, []byte(","))
	if len(parts) != 12 {
		return fmt.Errorf("expected length 12 but got %d", len(parts))
	}

	cur := 0
	for i := range m {
		var err error

		m[i][0], err = strconv.ParseFloat(string(parts[cur]), 64)
		if err != nil {
			return fmt.Errorf("could not parse float64: %v", err)
		}

		m[i][1], err = strconv.ParseFloat(string(parts[cur+1]), 64)
		if err != nil {
			return fmt.Errorf("could not parse float64: %v", err)
		}

		m[i][2], err = strconv.ParseFloat(string(parts[cur+2]), 64)
		if err != nil {
			return fmt.Errorf("could not parse float64: %v", err)
		}

		cur += 3
	}

	return nil
}

type Axis [3]float64

func (a Axis) MarshalText() ([]byte, error) {
	buf := bytes.Buffer{}
	for i, n := range a {
		if i != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString(strconv.FormatFloat(n, 'f', -1, 64))
	}
	return buf.Bytes(), nil
}

func (a *Axis) UnmarshalText(text []byte) error {
	parts := bytes.Split(text, []byte(","))
	if len(parts) != 3 {
		return fmt.Errorf("expected length 3 but got %d", len(parts))
	}

	var err error

	a[0], err = strconv.ParseFloat(string(parts[0]), 64)
	if err != nil {
		return fmt.Errorf("could not parse float64: %v", err)
	}

	a[1], err = strconv.ParseFloat(string(parts[1]), 64)
	if err != nil {
		return fmt.Errorf("could not parse float64: %v", err)
	}

	a[2], err = strconv.ParseFloat(string(parts[2]), 64)
	if err != nil {
		return fmt.Errorf("could not parse float64: %v", err)
	}

	return nil
}

type intList []int

func (l intList) MarshalText() ([]byte, error) {
	buf := bytes.Buffer{}
	for i, n := range l {
		if i != 0 {
			buf.WriteRune(',')
		}
		buf.WriteString(strconv.Itoa(n))
	}

	return buf.Bytes(), nil
}

func (l *intList) UnmarshalText(text []byte) error {
	parts := bytes.Split(text, []byte(","))

	items := []int{}
	for _, part := range parts {
		n, err := strconv.Atoi(string(part))
		if err != nil {
			return fmt.Errorf("could not parse int: %v", err)
		}
		items = append(items, n)
	}
	*l = items

	return nil
}

type Materials = intList
type BoneRefs = intList

type List[T any] struct {
	Items []T
}

type Camera struct {
	XMLName        xml.Name             `xml:"Camera"`
	RefId          int                  `xml:"refID,attr"`
	FOV            int                  `xml:"fieldOfView,attr"`
	Distance       float32              `xml:"distance,attr"`
	Transformation TransformationMatrix `xml:"transformation,attr"`
}

type Bone struct {
	RefId          int                  `xml:"refID,attr"`
	Transformation TransformationMatrix `xml:"transformation,attr"`
}

type Part struct {
	RefId     int       `xml:"refID,attr"`
	DesignId  int       `xml:"designID,attr"`
	Materials Materials `xml:"materials,attr"`
	Bone      Bone      `xml:"Bone"`
}

type Brick struct {
	XMLName  xml.Name `xml:"Brick"`
	RefId    int      `xml:"refID,attr"`
	DesignId int      `xml:"designID,attr"`
	Part     Part     `xml:"Part"`
}

type Rigid struct {
	XMLName        xml.Name             `xml:"Rigid"`
	RefId          int                  `xml:"refID,attr"`
	Transformation TransformationMatrix `xml:"transformation,attr"`
	BoneRefs       BoneRefs             `xml:"boneRefs,attr"`
}

type RigidRef struct {
	XMLName  xml.Name `xml:"RigidRef"`
	RigidRef int      `xml:"rigidRef,attr"`
	A        Axis     `xml:"a,attr"`
	Z        Axis     `xml:"z,attr"`
	T        Axis     `xml:"t,attr"`
}

type Joint struct {
	XMLName   xml.Name  `xml:"Joint"`
	Type      JointType `xml:"type,attr"`
	RigidRefs [2]RigidRef
}

type RigidSystem struct {
	XMLName xml.Name `xml:"RigidSystem"`
	Rigids  []Rigid
	Joints  []Joint
}

type GroupSystem struct {
	XMLName xml.Name `xml:"GroupSystem"`
}

type LXFML[M any] struct {
	XMLName      xml.Name          `xml:"LXFML"`
	VersionMajor int               `xml:"versionMajor,attr"`
	VersionMinor int               `xml:"versionMinor,attr"`
	Meta         M                 `xml:"Meta"`
	Cameras      List[Camera]      `xml:"Cameras"`
	Bricks       List[Brick]       `xml:"Bricks"`
	RigidSystems List[RigidSystem] `xml:"RigidSystems"`
	GroupSystems List[GroupSystem] `xml:"GroupSystems"`
}
