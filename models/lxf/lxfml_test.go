package lxf_test

import (
	"encoding/xml"
	"os"
	"slices"
	"testing"

	"github.com/I-Am-Dench/goverbuild/models/lxf"
)

var LddMeta = lxf.Meta{
	Application: lxf.MetaApplication{
		Name:         "LEGO Digital Designer",
		VersionMajor: 4,
		VersionMinor: 3,
	},
	Brand: lxf.MetaBrand{
		Name: "LDDExtended",
	},
	BrickSet: lxf.MetaBrickSet{
		Version: 1264,
	},
}

var Expected = lxf.LXFML[lxf.Meta]{
	VersionMajor: 5,
	Meta:         LddMeta,
	Cameras: []lxf.Camera{{
		RefId:    0,
		FOV:      80,
		Distance: 79.281982421875,
		Transformation: lxf.TransformationMatrix{
			{0.030392518267035484, 0, -0.99953794479370117},
			{-0.93535268306732178, 0.35257109999656677, -0.028440864756703377},
			{0.3524080216884613, 0.93578499555587769, 0.010715517215430737},
			{27.939624786376953, 74.190887451171875, 0.84954798221588135},
		},
	}},
	Bricks: []lxf.Brick{
		{
			RefId:    0,
			DesignId: 3062,
			Part: lxf.Part{
				RefId:     0,
				DesignId:  3062,
				Materials: lxf.Ints{44, 0},
				Bone: lxf.Bone{
					RefId: 0,
					Transformation: lxf.TransformationMatrix{
						{0.70710670948028564, 0, 0.7071068286895752},
						{0, 1, 0},
						{-0.7071068286895752, 0, 0.70710670948028564},
						{-0.79999923706054688, 1.5809999704360962, 3.200000524520874},
					},
				},
			},
		},
		{
			RefId:    1,
			DesignId: 2566,
			Part: lxf.Part{
				RefId:     1,
				DesignId:  2566,
				Materials: lxf.Ints{26},
				Bone: lxf.Bone{
					RefId: 1,
					Transformation: lxf.TransformationMatrix{
						{1, 0, 0},
						{0, 1, 0},
						{0, 0, 1},
						{-0.79999935626983643, -0.1390000581741333, 3.2000002861022949},
					},
				},
			},
		},
		{
			RefId:    2,
			DesignId: 4740,
			Part: lxf.Part{
				RefId:     2,
				DesignId:  4740,
				Materials: lxf.Ints{26, 0},
				Bone: lxf.Bone{
					RefId: 2,
					Transformation: lxf.TransformationMatrix{
						{0.7071068286895752, 0, -0.70710670948028564},
						{0, 1, 0},
						{0.70710670948028564, 0, 0.7071068286895752},
						{-0.7999987006187439, 2.5409998893737793, 3.1999998092651367},
					},
				},
			},
		},
		{
			RefId:    3,
			DesignId: 48729,
			Part: lxf.Part{
				RefId:     3,
				DesignId:  48729,
				Materials: lxf.Ints{26},
				Bone: lxf.Bone{
					RefId: 3,
					Transformation: lxf.TransformationMatrix{
						{0.99999988079071045, 0, 0},
						{0, 0, 1},
						{0, -1, 0},
						{-0.79999971389770508, 0.58100003004074097, 2.0000002384185791},
					},
				},
			},
		},
		{
			RefId:    4,
			DesignId: 32064,
			Part: lxf.Part{
				RefId:     4,
				DesignId:  32064,
				Materials: lxf.Ints{194},
				Bone: lxf.Bone{
					RefId: 4,
					Transformation: lxf.TransformationMatrix{
						{1, 0, 0},
						{0, 1, 0},
						{0, 0, 1},
						{-1.2000000476837158, 0.0010000000474974513, 2},
					},
				},
			},
		},
	},
	RigidSystems: []lxf.RigidSystem{
		{
			Rigids: []lxf.Rigid{
				{
					RefId:    0,
					BoneRefs: lxf.Ints{0, 1, 2},
					Transformation: lxf.TransformationMatrix{
						{0.70710670948028564, 0, 0.7071068286895752},
						{0, 1, 0},
						{-0.7071068286895752, 0, 0.70710670948028564},
						{-0.79999923706054688, 1.5809999704360962, 3.200000524520874},
					},
				},
				{
					RefId:    1,
					BoneRefs: lxf.Ints{3},
					Transformation: lxf.TransformationMatrix{
						{0.99999988079071045, 0, 0},
						{0, 0, 1},
						{0, -1, 0},
						{-0.79999971389770508, 0.58100003004074097, 2.0000002384185791},
					},
				},
				{
					RefId:    2,
					BoneRefs: lxf.Ints{4},
					Transformation: lxf.TransformationMatrix{
						{1, 0, 0},
						{0, 1, 0},
						{0, 0, 1},
						{-1.2000000476837158, 0.0010000000474974513, 2},
					},
				},
			},
			Joints: []lxf.Joint{
				{
					Type: lxf.JointTypeHinge,
					RigidRefs: [2]lxf.RigidRef{
						{
							RigidRef: 1,
							A:        lxf.Axis{0, 0, 1},
							Z:        lxf.Axis{1, 0, 0},
							T:        lxf.Axis{0, 1.2000000476837158, -0.1600000411272049},
						},
						{
							RigidRef: 0,
							A:        lxf.Axis{0, -1, 0},
							Z:        lxf.Axis{-1, 0, 0},
							T:        lxf.Axis{0, -0.84000003337860107, 0},
						},
					},
				},
				{
					Type: lxf.JointTypeHinge,
					RigidRefs: [2]lxf.RigidRef{
						{
							RigidRef: 1,
							A:        lxf.Axis{0, 1, 0},
							Z:        lxf.Axis{1, 0, 0},
							T:        lxf.Axis{0, 0.39999991655349731, 0},
						},
						{
							RigidRef: 2,
							A:        lxf.Axis{0, 0, 1},
							Z:        lxf.Axis{1, 0, 0},
							T:        lxf.Axis{0.40000000596046448, 0.57999998331069946, 0.40000000596046448},
						},
					},
				},
			},
		},
	},
	GroupSystems: []lxf.GroupSystem{{}},
}

func checkMeta(t *testing.T, expected, actual lxf.Meta) {
	if expected.Application.Name != actual.Application.Name {
		t.Errorf("meta: expected application %s but got %s", expected.Application.Name, actual.Application.Name)
	}

	if expected.Application.VersionMajor != actual.Application.VersionMajor || expected.Application.VersionMinor != actual.Application.VersionMinor {
		t.Errorf("meta: expected version %d.%d but got %d.%d", expected.Application.VersionMajor, expected.Application.VersionMinor, actual.Application.VersionMajor, actual.Application.VersionMinor)
	}

	if expected.Brand.Name != actual.Brand.Name {
		t.Errorf("meta: expected brand %s but got %s", expected.Brand.Name, actual.Brand.Name)
	}

	if expected.BrickSet.Version != actual.BrickSet.Version {
		t.Errorf("meta: expected brick set version %d but got %d", expected.BrickSet.Version, actual.BrickSet.Version)
	}
}

func equalMatrices(a, b lxf.TransformationMatrix) bool {
	for i := range a {
		if !slices.Equal(a[i][:], b[i][:]) {
			return false
		}
	}
	return true
}

func checkBrick(t *testing.T, expected, actual lxf.Brick) {
	if expected.RefId != actual.RefId {
		t.Errorf("brick: expected refID %d but got %d", expected.RefId, actual.RefId)
	}

	if expected.DesignId != actual.DesignId {
		t.Errorf("brick: expected designID %d but got %d", expected.DesignId, actual.DesignId)
	}

	if expected.Part.RefId != actual.Part.RefId {
		t.Errorf("brick: part: expected refID %d but got %d", expected.Part.RefId, actual.Part.RefId)
	}

	if expected.Part.DesignId != actual.Part.DesignId {
		t.Errorf("brick: part: expected refID %d but got %d", expected.Part.DesignId, actual.Part.DesignId)
	}

	if !slices.Equal(expected.Part.Materials, actual.Part.Materials) {
		t.Errorf("brick: part: expected materials %v but got %v", expected.Part.Materials, actual.Part.Materials)
	}

	if expected.Part.Bone.RefId != actual.Part.Bone.RefId {
		t.Errorf("brick: part: bone: expected refID %d but got %d", expected.Part.Bone.RefId, actual.Part.Bone.RefId)
	}

	if !equalMatrices(expected.Part.Bone.Transformation, actual.Part.Bone.Transformation) {
		t.Errorf("brick: part: bone: expected transformation %v but got %v", expected.Part.Bone.Transformation, actual.Part.Bone.Transformation)
	}
}

func checkRigid(t *testing.T, expected, actual lxf.Rigid) {
	if expected.RefId != actual.RefId {
		t.Errorf("rigid system: rigid: expected refID %d but got %d", expected.RefId, actual.RefId)
	}

	if !equalMatrices(expected.Transformation, actual.Transformation) {
		t.Errorf("rigid system: rigid: expected transformation %v but got %v", expected.Transformation, actual.Transformation)
	}

	if !slices.Equal(expected.BoneRefs, actual.BoneRefs) {
		t.Errorf("rigid system: rigid: expected boneRefs %v but got %v", expected.BoneRefs, actual.BoneRefs)
	}
}

func checkJoints(t *testing.T, expected, actual lxf.Joint) {
	if expected.Type != actual.Type {
		t.Errorf("rigid system: joint: expected type %v but got %v", expected.Type, actual.Type)
	}

	for i, a := range expected.RigidRefs {
		b := actual.RigidRefs[i]

		if a.RigidRef != b.RigidRef {
			t.Errorf("rigid system: joint: expected rigidRef %d but got %d", a.RigidRef, b.RigidRef)
		}

		if !slices.Equal(a.A[:], b.A[:]) {
			t.Errorf("rigid system: joint: expected axis (A) %v but got %v", a.A, b.A)
		}

		if !slices.Equal(a.Z[:], b.Z[:]) {
			t.Errorf("rigid system: joint: expected axis (Z) %v but got %v", a.Z, b.Z)
		}

		if !slices.Equal(a.T[:], b.T[:]) {
			t.Errorf("rigid system: joint: expected axis (T) %v but got %v", a.T, b.T)
		}
	}
}

func checkRigidSystem(t *testing.T, expected, actual lxf.RigidSystem) {
	if len(expected.Rigids) != len(actual.Rigids) {
		t.Errorf("rigid system: expected %d systems but got %d", len(expected.Rigids), len(actual.Joints))
	} else {
		for i := range expected.Rigids {
			checkRigid(t, expected.Rigids[i], actual.Rigids[i])
		}
	}

	if len(expected.Joints) != len(actual.Joints) {
		t.Errorf("rigid system: expected %d joints but got %d", len(expected.Joints), len(actual.Joints))
	} else {
		for i := range expected.Joints {
			checkJoints(t, expected.Joints[i], actual.Joints[i])
		}
	}
}

func CheckLXFML(t *testing.T, expected, actual lxf.LXFML[lxf.Meta]) {
	if expected.VersionMajor != actual.VersionMajor || expected.VersionMinor != actual.VersionMinor {
		t.Errorf("expected version %d.%d but got %d.%d", expected.VersionMajor, expected.VersionMinor, actual.VersionMajor, actual.VersionMinor)
	}

	checkMeta(t, expected.Meta, actual.Meta)

	if len(expected.Bricks) != len(actual.Bricks) {
		t.Errorf("expected %d bricks but got %d", len(expected.Bricks), len(actual.Bricks))
	} else {
		for i, expected := range expected.Bricks {
			checkBrick(t, expected, actual.Bricks[i])
		}
	}

	if len(expected.RigidSystems) != len(actual.RigidSystems) {
		t.Errorf("expected %d rigid systems but got %d", len(expected.RigidSystems), len(actual.RigidSystems))
	} else {
		for i, expected := range expected.RigidSystems {
			checkRigidSystem(t, expected, actual.RigidSystems[i])
		}
	}
}

func TestUnmarshal(t *testing.T) {
	data, err := os.ReadFile("testdata/model.lxfml")
	if err != nil {
		t.Fatal(err)
	}

	actual := lxf.LXFML[lxf.Meta]{}
	if err := xml.Unmarshal(data, &actual); err != nil {
		t.Fatal(err)
	}

	CheckLXFML(t, Expected, actual)
}
