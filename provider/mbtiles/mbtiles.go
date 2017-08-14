package mbtiles

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"errors"
	"io/ioutil"
	"path"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/mvt/vector_tile"
	"github.com/terranodo/tegola/util/dict"
)

// layer holds information about a query.
type layer struct {
	// The Name of the layer
	Name string
	// The ID field name, this will default to 'id' if not set to something other then empty string.
	IDFieldName string
}

// Provider provides the postgis data provider.
type Provider struct {
	db   *sql.DB
	meta *metaData
}

const Name = "mbtiles"

const (
	ConfigKeyFilename    = "filename"
	ConfigKeyLayers      = "layers"
	ConfigKeyFields      = "fields"
	ConfigKeyGeomIDField = "id_fieldname"
)

func init() {
	provider.Register(Name, NewProvider)
}

func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	// Validate the config to make sure it has the values I care about and the types for those values.
	c := dict.M(config)

	filename, err := c.String(ConfigKeyFilename, nil)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}

	meta, err := getMetaData(db)
	if err != nil {
		return nil, err
	}

	p := Provider{
		db:   db,
		meta: meta,
	}

	return p, nil
}

func (p Provider) LayerNames() []string {
	names := []string{p.meta.name}
	// if the name has a file extension, clean it up
	ext := path.Ext(p.meta.name)
	if len(ext) > 0 {
		name := path.Base(p.meta.name)
		names = append(names, name[0:len(name)-len(ext)])
	}

	return names
}

func (p Provider) MVTLayer(layerName string, tile tegola.Tile, tags map[string]interface{}) (layer *mvt.Layer, err error) {
	layer = new(mvt.Layer)
	t, err := getTile(p.db, tile.Z, tile.X, tile.Y)
	if err != nil {
		return layer, err
	}

	if isGzip(t.data) {
		r, err := gzip.NewReader(bytes.NewReader(t.data))
		if err != nil {
			return nil, err
		}
		t.data, err = ioutil.ReadAll(r)
		r.Close()
		if err != nil {
			return nil, err
		}
	}

	var v vectorTile.Tile
	if err := proto.Unmarshal(t.data, &v); err != nil {
		return nil, err
	}

	mvtTile, err := mvt.TileFromVTile(&v)
	if err != nil {
		return nil, err
	}

	for _, l := range mvtTile.Layers() {
		if l.Name == layerName {
			*layer = l
		}
	}

	return layer, nil
}

func isGzip(b []byte) bool {
	// magic number 1f 8b is from wikipedia
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

type bounds struct {
	left   float64
	bottom float64
	right  float64
	top    float64
}

type metaData struct {
	name        string
	mapType     string
	version     int64
	description string
	format      string
	bounds      *bounds
	attribution *string
}

func getMetaData(db *sql.DB) (*metaData, error) {
	rows, err := db.Query("SELECT name, value FROM metadata")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var meta metaData
	for rows.Next() {
		var name string
		var value sql.RawBytes
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}

		if name == "name" {
			meta.name = string(value)
		}

		if name == "type" {
			meta.mapType = string(value)
		}

		if name == "version" {
			meta.version, err = strconv.ParseInt(string(value), 10, 64)
			if err != nil {
				return nil, err
			}
		}

		if name == "description" {
			meta.description = string(value)
		}

		if name == "format" {
			meta.format = string(value)
		}

		if name == "bounds" {
			b := strings.Split(string(value), ",")
			if len(b) != 4 {
				return nil, errors.New("invalid bounds")
			}

			fn := func(n string) float64 {
				var f float64
				if err != nil {
					return f
				}

				f, err = strconv.ParseFloat(n, 64)
				return f
			}

			meta.bounds = &bounds{
				left:   fn(b[0]),
				bottom: fn(b[1]),
				right:  fn(b[2]),
				top:    fn(b[3]),
			}

			if err != nil {
				return nil, err
			}
		}

		if name == "attribution" {
			a := string(value)
			meta.attribution = &a
		}
	}

	return &meta, nil
}

type tile struct {
	zoom   int64
	column int64
	row    int64
	data   []byte
}

func getTile(db *sql.DB, zoom, column, row int) (*tile, error) {
	r := db.QueryRow("SELECT zoom_level, tile_column, tile_row, tile_data FROM tiles WHERE zoom_level=? AND tile_column=? AND tile_row=?", zoom, column, row)

	var t tile
	err := r.Scan(&t.zoom, &t.column, &t.row, &t.data)
	if err != nil {
		if err == sql.ErrNoRows {
			return &t, nil
		}
		return nil, err
	}

	return &t, nil
}
