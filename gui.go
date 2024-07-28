package main

import (
	"image"
	"image/color"
	"log"
	"math"
	"os"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type Plot struct {
	Points []f32.Point
}

func loadUi(datapoints [][]float64) {
	go func() {
		w := new(app.Window)
		w.Option(app.Title("OTDR Viewer"))
		w.Option(app.Size(unit.Dp(800), unit.Dp(600)))

		err := run(datapoints, w)
		if err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func run(datapoints [][]float64, w *app.Window) error {
	plot := &Plot{
		Points: normalizePoints(datapoints),
	}

	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			drawPlot(gtx, plot)
			e.Frame(gtx.Ops)
		}
	}
}

func drawPlot(gtx layout.Context, plot *Plot) layout.Dimensions {
	rect := image.Rect(100, 50, gtx.Constraints.Max.X-50, gtx.Constraints.Max.Y-50)
	paint.FillShape(gtx.Ops, color.NRGBA{R: 230, G: 230, B: 230, A: 255}, clip.Rect(rect).Op())

	// Draw axes
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(rect.Min.X), float32(rect.Min.Y)))
	path.LineTo(f32.Pt(float32(rect.Min.X), float32(rect.Max.Y)))
	path.LineTo(f32.Pt(float32(rect.Max.X), float32(rect.Max.Y)))
	paint.FillShape(gtx.Ops, color.NRGBA{A: 255}, clip.Stroke{
		Path:  path.End(),
		Width: 2,
	}.Op())

	// Draw line plot
	if len(plot.Points) > 1 {
		var path clip.Path
		path.Begin(gtx.Ops)

		scaledPoints := scalePlotPoints(plot.Points, rect)

		path.MoveTo(scaledPoints[0])
		for _, p := range scaledPoints[1:] {
			path.LineTo(p)
		}

		paint.FillShape(gtx.Ops, color.NRGBA{R: 255, A: 255}, clip.Stroke{
			Path:  path.End(),
			Width: 2,
		}.Op())
	}

	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func scalePlotPoints(points []f32.Point, rect image.Rectangle) []f32.Point {
	scaled := make([]f32.Point, len(points))
	xOffset := float32(rect.Dx()) * 0.05 // 10% offset
	for i, p := range points {
		scaled[i] = f32.Point{
			X: float32(rect.Min.X) + xOffset + p.X*float32(rect.Dx()-int(xOffset)),
			Y: float32(rect.Max.Y) - p.Y*float32(rect.Dy()),
		}
	}
	return scaled
}

func normalizePoints(data [][]float64) []f32.Point {
	if len(data) == 0 {
		return nil
	}

	points := make([]f32.Point, len(data))
	minX, maxX := data[0][0], data[0][0]
	minY, maxY := data[0][1], data[0][1]

	for _, p := range data {
		x, y := p[0], p[1]
		minX = math.Min(minX, x)
		maxX = math.Max(maxX, x)
		minY = math.Min(minY, y)
		maxY = math.Max(maxY, y)
	}

	rangeX := maxX - minX
	rangeY := maxY - minY

	for i, p := range data {
		points[i] = f32.Point{
			X: float32((p[0] - minX) / rangeX),
			Y: float32((p[1] - minY) / rangeY),
		}
	}

	return points
}

func PP(dataPoints [][]float64) {
	// Create a new plot
	p := plot.New()

	// Set the title and axis labels
	p.Title.Text = "Large Dataset Plot"
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Y"

	// Create the data points
	pts := make(plotter.XYs, len(dataPoints)) // Adjust this to your actual data size
	for i := range pts {
		// Replace this with your actual data
		pts[i].X = dataPoints[i][0]
		pts[i].Y = dataPoints[i][1]
	}

	// Create a line plotter and set its style
	line, err := plotter.NewLine(pts)
	if err != nil {
		log.Fatalf("Could not create line plotter: %v", err)
	}
	line.Color = color.RGBA{R: 255, A: 255}

	// Add the line plotter to the plot
	p.Add(line)

	// Save the plot to a PNG file
	if err := p.Save(8*vg.Inch, 4*vg.Inch, "plot.png"); err != nil {
		log.Fatalf("Could not save plot: %v", err)
	}
}
