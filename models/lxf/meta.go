package lxf

type MetaApplication struct {
	Name         string `xml:"name,attr"`
	VersionMajor int    `xml:"versionMajor,attr"`
	VersionMinor int    `xml:"versionMinor,attr"`
}

type MetaBrand struct {
	Name string `xml:"name,attr"`
}

type MetaBrickSet struct {
	Version int `xml:"version,attr"`
}

type Meta struct {
	Application MetaApplication `xml:"Application"`
	Brand       MetaBrand       `xml:"Brand"`
	BrickSet    MetaBrickSet    `xml:"BrickSet"`
}

var LegoUniverseMeta = Meta{
	Application: MetaApplication{
		Name: "LEGO Universe",
	},
	Brand: MetaBrand{
		Name: "LEGOUniverse",
	},
	BrickSet: MetaBrickSet{
		Version: 457,
	},
}
