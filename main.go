package main

import (
	"context"
	"fmt"
	"math"

	"github.com/jmoiron/sqlx"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkt"
	"github.com/paulmach/orb/geo"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/maptile"
)

func main() {
	fmt.Println("bound:")
	bound()

	fmt.Println("tiles:")
	tiles()
}

func bound() {
	nePoint := orb.Point{139.7809654260254, 35.698836016401685}
	swPoint := orb.Point{139.7468906427002, 35.67771329985728}

	bound := orb.MultiPoint{nePoint, swPoint}.Bound()

	c := geojson.NewFeatureCollection()
	f := geojson.NewFeature(bound)
	f.Properties["name"] = "皇居周辺"
	c.Append(f)
	b, _ := c.MarshalJSON()
	println(string(b))
	// => {"features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[139.7809654260254,35.698836016401685],[139.7468906427002,35.698836016401685],[139.7468906427002,35.67771329985728],[139.7809654260254,35.67771329985728],[139.7809654260254,35.698836016401685]]]},"properties":{"name":"皇居周辺"}}],"type":"FeatureCollection"}
}

func tiles() {
	nePoint := orb.Point{139.7809654260254, 35.698836016401685}
	swPoint := orb.Point{139.7468906427002, 35.67771329985728}
	zoom := maptile.Zoom(14)

	base := orb.MultiPoint{nePoint, swPoint}.Bound()

	tiles := []orb.Bound{}
	minTile := maptile.At(base.Min, zoom)
	maxTile := maptile.At(base.Max, zoom)
	minX, minY := float64(minTile.X), float64(minTile.Y)
	maxX, maxY := float64(maxTile.X), float64(maxTile.Y)
	for x := math.Min(minX, maxX); x <= math.Max(minX, maxX); x++ {
		for y := math.Min(minY, maxY); y <= math.Max(minY, maxY); y++ {
			tb := maptile.New(uint32(x), uint32(y), zoom).Bound()
			if !containBoundAny(base, tb) {
				continue
			}
			tiles = append(tiles, tb)
		}
	}

	db, _ := sqlx.ConnectContext(context.Background(), "mysql", "...")
	for _, t := range tiles {
		rows, err := db.NamedQuery(`
			SELECT *
				FROM jobs
				AND ST_Within(
					ST_GeomFromText(concat('POINT(', longitude, ' ', latitude, ')'), 4326, 'axis-order=long-lat'),
					ST_GeomFromText(:polygon, 4326, 'axis-order=long-lat')
				)
			`,
			map[string]any{
				"polygon": wkt.MarshalString(t.ToPolygon()),
			},
		)

		// do something
		_ = rows
		_ = err
	}

	c := geojson.NewFeatureCollection()
	for _, t := range tiles {
		c.Append(geojson.NewFeature(t))
	}
	b, _ := c.MarshalJSON()
	println(string(b))
	// => {"features":[{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[139.74609375,35.69299463209881],[139.76806640625,35.69299463209881],[139.76806640625,35.71083783530009],[139.74609375,35.71083783530009],[139.74609375,35.69299463209881]]]},"properties":null},{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[139.74609375,35.67514743608467],[139.76806640625,35.67514743608467],[139.76806640625,35.69299463209881],[139.74609375,35.69299463209881],[139.74609375,35.67514743608467]]]},"properties":null},{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[139.76806640625,35.69299463209881],[139.7900390625,35.69299463209881],[139.7900390625,35.71083783530009],[139.76806640625,35.71083783530009],[139.76806640625,35.69299463209881]]]},"properties":null},{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[139.76806640625,35.67514743608467],[139.7900390625,35.67514743608467],[139.7900390625,35.69299463209881],[139.76806640625,35.69299463209881],[139.76806640625,35.67514743608467]]]},"properties":null}],"type":"FeatureCollection"}
}

func containBoundAny(base, target orb.Bound) bool {
	for _, p := range []orb.Point{target.LeftTop(), {target.Right(), target.Top()}, target.RightBottom(), {target.Left(), target.Bottom()}} {
		if base.Contains(p) {
			return true
		}
	}
	return false
}

const maxZoomLevel = 22

func tileWithZoom() {
	nePoint := orb.Point{139.7809654260254, 35.698836016401685}
	swPoint := orb.Point{139.7468906427002, 35.67771329985728}
	zoom := maptile.Zoom(14)

	base := orb.MultiPoint{nePoint, swPoint}.Bound()

	// 画面領域に収まるzoom levelでタイルを作る
	nb, ok := nextZoomBound(base, zoom)
	if ok {
		for {
			// これ以上zoom出来ない
			if maxZoomLevel <= zoom {
				break
			}

			currentZoomTile := maptile.At(base.Center(), zoom).Bound()
			if containBoundSize(nb, currentZoomTile) {
				break
			}
			zoom = zoom + 1
		}
	}

	// 決定されたzoomを元にtilesを生成する
}

// zoom levelごとの表示できるメートル/pixel定義
var meterPixelZoom = map[int]float64{
	0:  123456.79012,
	1:  59959.436,
	2:  29979.718,
	3:  14989.859,
	4:  7494.929,
	5:  3747.465,
	6:  1873.732,
	7:  936.866,
	8:  468.433,
	9:  234.217,
	10: 117.108,
	11: 58.554,
	12: 29.277,
	13: 14.639,
	14: 7.319,
	15: 3.660,
	16: 1.830,
	17: 0.915,
	18: 0.457,
	19: 0.229,
	20: 0.114,
	21: 0.057,
	22: 0.029,
}

func nextZoomBound(b orb.Bound, currentZoom maptile.Zoom) (_ orb.Bound, ok bool) {
	if currentZoom >= maxZoomLevel {
		return orb.Bound{}, false
	}

	// 現在のboundsの辺とzoom levelからpixel概算値を出す
	xLs := orb.LineString{b.LeftTop(), orb.Point{b.Right(), b.Top()}}
	yLs := orb.LineString{b.LeftTop(), orb.Point{b.Left(), b.Bottom()}}
	xDistance := geo.Distance(xLs[0], xLs[1])
	yDistance := geo.Distance(yLs[0], yLs[1])
	xPixel := xDistance / meterPixelZoom[int(currentZoom)]
	yPixel := yDistance / meterPixelZoom[int(currentZoom)]

	// pixel値とメートル/pixel定数を元に次のzoom levelでのboundsを出す
	nextZoom := currentZoom + 1
	nePoint := geo.PointAtBearingAndDistance(xLs[0], geo.Bearing(xLs[0], xLs[1]), xPixel*meterPixelZoom[int(nextZoom)])
	swPoint := geo.PointAtBearingAndDistance(yLs[0], geo.Bearing(yLs[0], yLs[1]), yPixel*meterPixelZoom[int(nextZoom)])
	return orb.MultiPoint{b.LeftTop(), nePoint, swPoint, {nePoint.X(), swPoint.Y()}}.Bound(), true
}

// containBoundSize baseの面積にtaregetが完全に含まれるかどうか
func containBoundSize(base, target orb.Bound) bool {
	baseXl := geo.Distance(base.LeftTop(), orb.Point{base.Right(), base.Top()})
	targetXl := geo.Distance(target.LeftTop(), orb.Point{target.Right(), target.Top()})
	if baseXl < targetXl {
		return false
	}

	baseYl := geo.Distance(base.LeftTop(), orb.Point{base.Left(), base.Bottom()})
	targetYl := geo.Distance(target.LeftTop(), orb.Point{target.Left(), target.Bottom()})
	return baseYl > targetYl
}
