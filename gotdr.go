/*
Author: Naseredin Aramnejad naseredin.aramnejad@gmail.com
This script is designed to extract all the possible information from the
given sor file.
each sor file (Provided by OTDR Equipment) contains multiple data blocks.
Formulas and blueprint of this script are inspired by the information provided by:
Sidney Li
http://morethanfootnotes.blogspot.com/2015/07/
*/

package main

import (
	"bytes"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

const lightSpeed = 299.79181901 // m/µsec

func nukeIfErr(err error) {
	if err != nil {
		log.Fatalln(err.Error())
	}
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		fmt.Printf("Failed to open browser: %v\n", err)
	}
}

func mod(a, b float64) float64 {
	return a - b*math.Floor(a/b)
}

func removePaths(stack []byte) []byte {
	lines := bytes.Split(stack, []byte("\n"))
	for i, line := range lines {
		if idx := bytes.LastIndex(line, []byte("/go/")); idx != -1 {
			lines[i] = line[idx+4:]
		} else if idx := bytes.Index(line, []byte(":")); idx != -1 {
			lines[i] = line[idx:]
		}
	}
	return bytes.Join(lines, []byte("\n"))
}

func customPanicHandler() {
	if r := recover(); r != nil {
		// Get the stack trace
		stack := debug.Stack()

		// Remove file paths from the stack trace
		sanitizedStack := removePaths(stack)

		// Log or print the sanitized stack trace
		fmt.Printf("Panic: %v\n%s", r, sanitizedStack)

		// Optionally, exit the program
		os.Exit(1)
	}
}

// Reverse will reverse the hex string in every 2 bytes. Example: 0ABCD123 => 23D1BC0A.
func Reverse(s string) string {
	str := ""
	for ind := 0; ind < len(s); ind += 2 {
		str = s[ind:ind+2] + str
	}
	return str
}

func dB(point int64) float64 {
	return float64(point*-1000) * math.Pow(10, -6)
}

func ReadSorFile(filename string) otdrRawData {

	r := otdrRawData{
		Filename: filename,
	}

	f, err := os.Open(filename)
	if err != nil {
		nukeIfErr(err)
	}
	defer f.Close()

	// Get the file size
	stat, err := f.Stat()
	if err != nil {
		nukeIfErr(err)
	}

	// Read the entire file at once
	buffer := make([]byte, stat.Size())
	_, err = io.ReadFull(f, buffer)
	if err != nil {
		nukeIfErr(err)
	}

	//Converting the byte array into a hex String
	r.HexData = hex.EncodeToString(buffer)

	//Converting the HexData to a text string
	chars, err := hex.DecodeString(r.HexData)
	nukeIfErr(err)

	r.Decodedfile = string(chars)
	return r
}

// parsHexValue calls the Reverse() funcition to reverse the order of the provided HexString and then converts it's value to int64.
func parsHexValue(hexData string) int64 {
	output, err := strconv.ParseInt(Reverse(hexData), 16, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return output
}

func (d *otdrRawData) draw() {

	// Create a new line chart instance
	xValues := make([]opts.LineData, len(d.DataPoints))
	yValues := make([]opts.LineData, len(d.DataPoints))
	// yValues := make([]opts.ScatterData, len(d.DataPoints))

	for i, point := range d.DataPoints {
		xValues[i] = opts.LineData{Value: point[0]}
		yValues[i] = opts.LineData{Value: point[1]}
		// yValues[i] = opts.ScatterData{Value: point[1]}
	}

	// Create a new line chart instance
	line := charts.NewLine()
	// line := charts.NewScatter()

	// Set global options like title and legend
	line.SetGlobalOptions(
		charts.WithColorsOpts(opts.Colors{
			"green",
		}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show: opts.Bool(true),
			Feature: &opts.ToolBoxFeature{
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show:       opts.Bool(true),
					YAxisIndex: false,
				},
				Restore: &opts.ToolBoxFeatureRestore{
					Show: opts.Bool(true),
				},
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
			},
		}),

		charts.WithInitializationOpts(opts.Initialization{
			Width:  "1300px",
			Height: "500px",
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "inside",
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Type:       "slider",
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true)}),
		charts.WithAnimation(true),

		charts.WithTitleOpts(opts.Title{
			Title: "GOTDR Viewer",
		}),
	)

	markPoints := make([]opts.MarkPointNameCoordItem, 0, len(d.DataPoints))

	for _, ev := range d.Events {
		loc := d.return_index(ev.EventLocM)

		if loc[0] == 0 {
			continue
		}

		c := "blue"
		if strings.Contains(ev.EventType, "EXXX") || strings.Contains(ev.EventType, "E999") {
			c = "red"
		}

		mkpoint := opts.MarkPointNameCoordItem{
			Name:       ev.EventType,
			Value:      strconv.Itoa(ev.EventNumber),
			Coordinate: []interface{}{loc[0], loc[1]},
			Symbol:     "pin",
			ItemStyle: &opts.ItemStyle{
				Color:   c,
				Opacity: 0.5,
			},
			SymbolSize: 35,
		}
		markPoints = append(markPoints, mkpoint)

	}

	// Add data to the line chart
	line.SetXAxis(xValues).AddSeries("Reflection", yValues, charts.WithMarkPointNameCoordItemOpts(markPoints...))
	line.SetSeriesOptions(
		charts.WithMarkPointStyleOpts(
			opts.MarkPointStyle{Label: &opts.Label{Show: opts.Bool(true)}}),
		charts.WithAreaStyleOpts(opts.AreaStyle{
			Color:   "yellow",
			Opacity: 0.1,
		}),
	)

	f, _ := os.Create("graph.html")
	// line.Render(f)
	d.generateHTML(f, line)

	openBrowser("graph.html")
}

func (d *otdrRawData) generateHTML(w io.Writer, line *charts.Line) {
	w.Write([]byte(`
    <!DOCTYPE html>
    <html>
    <head>
        <meta charset="utf-8">
        <title>OTDR Report</title>
		<style>
* {
    box-sizing: border-box;
    margin: 0;
    padding: 0;
  }
  
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen-Sans, Ubuntu, Cantarell, "Helvetica Neue", sans-serif;
    background-color: #f0f0f0;
    font-size: 16px;
  }
  
  .container {
    display: flex;
    flex-direction: column;
    background-color: #ffffff;
    border-radius: 10px;
    padding: 20px;
    box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
    max-width: 1600px;
    margin: 0 auto;
  }
  
  .summary {
    width: 100%;
    background-color: #f9f9f9;
    box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
    border-radius: 5px;
    padding: 20px;
    margin-top: 20px;
  }
  
  .chart {
    width: 100%;
  }

  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 14px;
    table-layout: fixed;
  }
  
  th, td {
    border: 1px solid #e0e0e0;
    padding: 10px;
    text-align: left;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  
  th {
    background-color: #4a90e2;
    color: white;
    font-weight: bold;
  }
  
  tr:nth-child(even) {
    background-color: #f2f2f2;
  }
  
  /* Responsive design */
  @media (max-width: 768px) {
    .container {
      padding: 15px;
    }
  
    th, td {
      padding: 8px;
      font-size: 12px;
    }
  }
		</style>

    </head>
    <body>
        <div class="container">
            <div class="chart">
    `))

	line.Render(w)

	tmpl := template.Must(template.New("summary").Parse(`
 			</div>
            <div class="summary">
                <table>
                    <thead>
                        <tr>
                            <th>Fixed Parameters</th>
                            <th>Value</th>
                        </tr>
                    </thead>
                    <tbody>
                        <tr>
                            <td>Date & Time</td>
                            <td>{{.DT}}</td>
                        </tr>
                        <tr>
                            <td>Scan Range</td>
                            <td>{{.SR}} m</td>
                        </tr>
                        <tr>
                            <td>Unit</td>
                            <td>{{.UNIT}}</td>
                        </tr>
						<tr>
                            <td>Actual Wavelength</td>
                            <td>{{.WL}} nm</td>
                        </tr>
						<tr>
                            <td>Pulse Width QTY</td>
                            <td>{{.PWQ}}</td>
                        </tr>
						<tr>
                            <td>Pulse Width(ns)</td>
                            <td>{{.PW}}</td>
                        </tr>
						<tr>
                            <td>Sample Quantity</td>
                            <td>{{.SQ}}</td>
                        </tr>
						<tr>
                            <td>Fiber Light Speed</td>
                            <td>{{.FLS}} km/s</td>
                        </tr>
						<tr>
                            <td>Fiber Length (EOF)</td>
                            <td>{{.FLEN}} m</td>
                        </tr>
						<tr>
                            <td>Bellcore Version</td>
                            <td>{{.BLV}}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
			<div class="summary">
                <table>
                    <thead>
                        <tr>
                            <th>Key events</th>
                            <th>Value</th>
                        </tr>
                    </thead>
                    <tbody>
					<tr>
                        <td>Key events qty</td>
                        <td>{{.KE}}</td>
                    </tr>
					</tbody>
                </table>
            </div>
			<div class="summary">
                <table>
                    <thead>
                        <tr>
                            <th>Supplier Parameters</th>
                            <th>Value</th>
                        </tr>
                    </thead>
                    <tbody>
						<tr>
                            <td>OTDR Supplier</td>
                            <td>{{.OS}}</td>
                        </tr>
						<tr>
                            <td>OTDR Module Name</td>
                            <td>{{.OMN}}</td>
                        </tr>
						<tr>
                            <td>OTDR Name</td>
                            <td>{{.ON}}</td>
                        </tr>
						<tr>
                            <td>OTDR Software Version</td>
                            <td>{{.OSV}}</td>
                        </tr>
						<tr>
                            <td>OTDR Module Serial Number Version</td>
                            <td>{{.OMS}}</td>
                        </tr>
						<tr>
                            <td>OTDR other info</td>
                            <td>{{.OOI}}</td>
                        </tr>
                    </tbody>
                </table>
            </div>
        </div>
    </body>
    </html>
`))

	data := struct {
		DT   time.Time
		UNIT string
		WL   float64
		PWQ  int64
		FLS  float64
		PW   []int64
		SQ   []int64
		FLEN float64
		BLV  float64
		OS   string
		ON   string
		OSV  string
		OMS  string
		OOI  string
		OMN  string
		SR   []float64
		KE   int
	}{
		DT:   d.FixedParams.DateTime,
		UNIT: d.FixedParams.Unit,
		WL:   d.FixedParams.ActualWL,
		PWQ:  d.FixedParams.PulseWidthNo,
		FLS:  d.FixedParams.FiberSpeed * 1000,
		PW:   d.FixedParams.PulseWidth,
		SQ:   d.FixedParams.SampleQTY,
		FLEN: d.TotalLength,
		SR:   d.FixedParams.Range,
		BLV:  d.BellCoreVersion,
		ON:   d.Supplier.OTDRName,
		OMN:  d.Supplier.OTDRModuleName,
		OS:   d.Supplier.OTDRSupplier,
		OSV:  d.Supplier.OTDRswVersion,
		OMS:  d.Supplier.OTDRModuleSN,
		OOI:  d.Supplier.OTDROtherInfo,
		KE:   len(d.Events),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Println("failed to create the html file")
	}
	htmlContent := buf.String()

	w.Write([]byte(htmlContent))
}

func (d *otdrRawData) return_index(loc float64) []float64 {
	closest := []float64{math.Inf(0), 0}

	for ind, i := range d.DataPoints {
		if i[0] == loc {
			return []float64{float64(ind), i[1]}
		}

		if math.Abs(loc-i[0]) < math.Abs(loc-closest[0]) && i[0] < loc {
			closest = []float64{float64(ind), i[1]}
		} else if i[0] > loc {
			break
		}
	}

	return []float64{closest[0], closest[1]}
}

func (d *otdrRawData) mapKeyEvents(events string) map[int][2]int {
	m := make(map[int][2]int)
	start := 4
	evnumbers := int(parsHexValue(events[:start]))

	for i := 1; i <= evnumbers; i++ {
		if i == evnumbers {
			m[i] = [2]int{start, len(events) - 46}
		} else {
			end := strings.Index(events[start+84:], fmt.Sprintf("%02x00", i+1))
			if end == -1 {
				fmt.Println("pattern not found:", events)
				return nil
			}
			end += start + 84
			m[i] = [2]int{start, end}
			start = end
		}
	}

	return m
}

func (d *otdrRawData) GetNext(key string) string {
	if _, exists := d.SecLocs[key]; !exists {
		log.Println(key, "not found")
		return ""
	}

	index := d.SecLocs[key][1]
	var nextKey string
	nextIndex := math.Inf(1)

	for k, v := range d.SecLocs {
		if k == key || len(v) < 2 {
			continue
		}
		if float64(index) < float64(v[1]) && float64(v[1]) < nextIndex {
			nextIndex = float64(v[1])
			nextKey = k
		}
	}

	return nextKey
}

func (d *otdrRawData) GetOrder() {
	sections := []string{
		"SupParams",
		"ExfoNewProprietaryBlock",
		"Map",
		"FxdParams",
		"YokogawaSpecial",
		"SetupParams",
		"DataPts",
		"NokiaParams",
		"KeyEvents",
		"GenParams",
		"WaveMTSParams",
		"WavetekTwoMTS",
		"WavetekThreeMTS",
		"BlocOtdrPrivate",
		"ActernaConfig",
		"ActernaMiniCurve",
		"AcqParam",
		"ViewParams",
		"SystemParams",
		"AnalysisParams",
		"MiscParams",
		"JDSUEvenementsMTS",
		"Cksum",
	}

	sectionLocations := make(map[string][]int)

	for _, word := range sections {
		re := regexp.MustCompile(regexp.QuoteMeta(word))
		matches := re.FindAllStringIndex(d.Decodedfile, -1)
		locations := make([]int, len(matches))
		for i, match := range matches {
			locations[i] = match[0]
		}
		sectionLocations[word] = locations
	}

	if len(sectionLocations["Cksum"]) < 2 {
		fmt.Printf("%s file has no checksum\n", d.Filename)
		os.Exit(1)
	}

	d.SecLocs = sectionLocations
}

// Under construction
func (d *otdrRawData) getSetupParams() {
	// s := SetupParams{}

	if len(d.SecLocs["SetupParams"]) == 0 {
		// log.Println("SetupParams section is missing")
		return
	}

	setupparams := d.HexData[(d.SecLocs["SetupParams"][1]+10)*2 : (d.SecLocs[d.GetNext("SetupParams")][1] * 2)]

	log.Println("setupParams", setupparams)
}

// Under construction
func (d *otdrRawData) getMiscParams() {
	m := MiscParams{}

	if len(d.SecLocs["MiscParams"]) == 0 {
		return
	}

	slicedMiscParams := strings.Split(d.Decodedfile[d.SecLocs["MiscParams"][1]+10:d.SecLocs[d.GetNext("MiscParams")][1]], "\x00")[1:]

	m.Mode = d.extractData(slicedMiscParams, 0)
	m.FiberType = d.extractData(slicedMiscParams, 1)

	d.MiscParams = m
}

// Under construction
func (d *otdrRawData) getAcqParam() {
	// a :=AcqParam{}

	if len(d.SecLocs["AcqParam"]) == 0 {
		// log.Println("AcqParam section is missing")
		return
	}

	acqparam := d.HexData[(d.SecLocs["AcqParam"][1]+10)*2 : (d.SecLocs[d.GetNext("AcqParam")][1] * 2)]

	log.Println("AcqParams", acqparam)
}

// Under construction
func (d *otdrRawData) getViewParams() {
	// v :=ViewParams{}

	if len(d.SecLocs["ViewParams"]) == 0 {
		// log.Println("ViewParams section is missing")
		return
	}

	viewparams := d.HexData[(d.SecLocs["ViewParams"][1]+10)*2 : (d.SecLocs[d.GetNext("ViewParams")][1] * 2)]

	log.Println("viewParams", viewparams)
}

// Under construction
func (d *otdrRawData) getAnalysisParams() {
	// a :=AnalysisParams{}

	if len(d.SecLocs["AnalysisParams"]) == 0 {
		// log.Println("AnalysisParams section is missing")
		return
	}

	analyticsparams := d.HexData[(d.SecLocs["AnalysisParams"][1]+10)*2 : (d.SecLocs[d.GetNext("AnalysisParams")][1] * 2)]

	log.Println("AnalyticsParams", analyticsparams)
}

// Under construction
func (d *otdrRawData) getSystemParams() {
	// s :=SystemParams{}

	if len(d.SecLocs["SystemParams"]) == 0 {
		// log.Println("SystemParams section is missing")
		return
	}

	systemparams := d.HexData[(d.SecLocs["SystemParams"][1]+10)*2 : (d.SecLocs[d.GetNext("SystemParams")][1] * 2)]

	log.Println("SystemParams", systemparams)
}

// getFixedParams function extracts the Fixed Parameters from the sor file and stores it in FixInfos struct.
func (d *otdrRawData) getFixedParams() {

	f := FixInfo{}

	if len(d.SecLocs["FxdParams"]) == 0 {
		log.Println("FxdParams section is missing")
		return
	}

	fixInfo := d.HexData[(d.SecLocs["FxdParams"][1]+10)*2 : (d.SecLocs[d.GetNext("FxdParams")][1] * 2)]
	p := 8

	f.DateTime = time.Unix(parsHexValue(fixInfo[:p]), 0)

	unit, err := hex.DecodeString(fixInfo[p : p+4])
	nukeIfErr(err)
	p += 4

	f.ActualWL = float64(parsHexValue(fixInfo[p:p+4])) / 10.0
	p += 4

	f.AO = float64(parsHexValue(fixInfo[p : p+8]))
	p += 8

	f.AOD = float64(parsHexValue(fixInfo[p : p+8]))
	p += 8

	f.PulseWidthNo = parsHexValue(fixInfo[p : p+4])
	p += 4

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.PulseWidth = append(f.PulseWidth, parsHexValue(fixInfo[p:p+4]))
		p += 4
	}

	resolution_m_p1 := []float64{}
	for i := 0; i < int(f.PulseWidthNo); i++ {
		resolution_m_p1 = append(resolution_m_p1, float64(parsHexValue(fixInfo[p:p+8]))*math.Pow(10, -8))
		p += 8
	}

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.SampleQTY = append(f.SampleQTY, parsHexValue(fixInfo[p:p+8]))
		p += 8
	}

	f.IOR = float64(parsHexValue(fixInfo[p : p+8]))
	p += 8

	f.RefIndex = f.IOR * math.Pow(10, -5)

	f.FiberSpeed = lightSpeed / f.RefIndex

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.Resolution = append(f.Resolution, resolution_m_p1[i]*f.FiberSpeed)
	}

	f.Backscattering = float64(parsHexValue(fixInfo[p:p+4])) * -0.1
	p += 4

	f.Averaging = parsHexValue(fixInfo[p : p+8])
	p += 8

	f.AveragingTime = float64(parsHexValue(fixInfo[p:p+4])) / 600
	p += 4

	for i := 0; i < int(f.PulseWidthNo); i++ {
		f.Range = append(f.Range, float64(f.SampleQTY[i])*f.Resolution[i])
	}

	f.Unit = string(unit)

	d.FixedParams = f
}

func (d *otdrRawData) getDataPoints() {

	if len(d.SecLocs["DataPts"]) == 0 {
		log.Println("DataPts section is missing")
		return
	}
	dtpoints := d.HexData[d.SecLocs["DataPts"][1]*2 : d.SecLocs[d.GetNext("DataPts")][1]*2][40:]
	var start int64 = 0
	var cumulative_length float64 = 0

	for i := range d.FixedParams.SampleQTY {

		qty := int64(d.FixedParams.SampleQTY[i])
		resolution := d.FixedParams.Resolution[i]

		var j int64
		for j = 0; j < qty; j++ {
			index := start + j
			if index < int64(len(dtpoints)) {
				hex_value := dtpoints[index*4 : index*4+4]
				db_value := math.Round(dB(parsHexValue(hex_value))*1000) / 1000
				passedlen := math.Round(float64(cumulative_length*1000)) / 1000
				dataPoint := []float64{passedlen, db_value}
				d.DataPoints = append(d.DataPoints, dataPoint)
				cumulative_length += resolution
			}
		}
		start += qty
	}
}

// SupParams function extracts the Supplier Parameters from the sor file and stores it in SupParam struct.
func (d *otdrRawData) getSupParams() {

	if len(d.SecLocs["SupParams"]) == 0 {
		log.Println("SupParams section is missing")
		return
	}

	supString := strings.Split(d.Decodedfile[d.SecLocs["SupParams"][1]+10:d.SecLocs[d.GetNext("SupParams")][1]], "\x00")
	slicedParams := supString[:len(supString)-1]

	supInfo := SupParam{
		OTDRSupplier:   strings.TrimSpace(slicedParams[0]),
		OTDRName:       strings.TrimSpace(slicedParams[1]),
		OTDRsn:         strings.TrimSpace(slicedParams[2]),
		OTDRModuleName: strings.TrimSpace(slicedParams[3]),
		OTDRModuleSN:   strings.TrimSpace(slicedParams[4]),
		OTDRswVersion:  strings.TrimSpace(slicedParams[5]),
		OTDROtherInfo:  strings.TrimSpace(slicedParams[6]),
	}

	d.Supplier = supInfo
}

func (d *otdrRawData) extractData(data []string, item int) string {
	if len(data)-1 < item {
		return ""
	} else {
		return strings.TrimSpace(data[item])
	}
}

// GenParams function extracts the General Parameters from the sor file and stores it in GenParam struct.
func (d *otdrRawData) getGenParams() {

	if len(d.SecLocs["GenParams"]) == 0 {
		log.Println("GenParams section is missing")
		return
	}

	genStringBeforeSplit := strings.Split(d.Decodedfile[d.SecLocs["GenParams"][1]+10:d.SecLocs[d.GetNext("GenParams")][1]], "\x00")
	genString := genStringBeforeSplit[:len(genStringBeforeSplit)-1]

	genInfo := GenParam{
		CableID:        d.extractData(genString, 0)[2:],
		Lang:           d.extractData(genString, 0)[:2],
		FiberID:        d.extractData(genString, 1),
		LocationA:      d.extractData(genString, 2)[4:],
		LocationB:      d.extractData(genString, 3),
		CableCode:      d.extractData(genString, 4),
		BuildCondition: d.extractData(genString, 5),
		Operator:       d.extractData(genString, 13),
		Comment:        d.extractData(genString, 14),
		FiberType:      "G." + strconv.FormatInt(parsHexValue(hex.EncodeToString([]byte(d.extractData(genString, 2)[:2]))), 10),
		OTDRWavelength: strconv.FormatInt(parsHexValue(hex.EncodeToString([]byte(d.extractData(genString, 2)[2:4]))), 10) + " nm",
	}

	d.GenParams = genInfo
}

// getFiberLength calculates the fiber length and returns it.
func (d *otdrRawData) getFiberLength() {
	for _, v := range d.Events {
		if strings.Contains(v.EventType, "EXX") || strings.Contains(v.EventType, "E99") {
			d.TotalLength = float64(v.EventLocM)
		}
	}
}

// getBellCoreVersion reads the bellcore version from the sor file and returns it.
func (d *otdrRawData) getBellCoreVersion() {

	if len(d.SecLocs["Map"]) == 0 {
		log.Println("Map section is missing")
		return
	}
	d.BellCoreVersion = float64(parsHexValue(d.HexData[(d.SecLocs["Map"][0]+4)*2:(d.SecLocs["Map"][0]+5)*2])) / 100.0
}

// getTotalLoss reads the total loss of the fiber from the sor file and returns it.
func (d *otdrRawData) getTotalLoss() {

	if len(d.SecLocs["WaveMTSParams"]) > 0 {
		totallossinfo := d.HexData[(d.SecLocs["WaveMTSParams"][1]-22)*2 : (d.SecLocs["WaveMTSParams"][1]-18)*2]
		d.TotalLoss = float64(parsHexValue(totallossinfo)) * 0.001
	} else {
		d.TotalLoss = 0
	}
}

// getKeyEvents function extracts the events information from the sor file and stores it in OTDREvent struct.
func (d *otdrRawData) getKeyEvents() {

	d.Events = map[int]OTDREvent{}

	if len(d.SecLocs["KeyEvents"]) == 0 {
		log.Println("KeyEvents section is missing")
		return
	}
	events := d.HexData[(d.SecLocs["KeyEvents"][1]+10)*2 : d.SecLocs[d.GetNext("KeyEvents")][1]*2]
	p := d.mapKeyEvents(events)

	var eventhexlist []string
	for _, v := range p {
		eventhexlist = append(eventhexlist, events[v[0]:v[1]])
	}

	for _, e := range eventhexlist {

		event := OTDREvent{}
		eNum := int(parsHexValue(e[:4]))
		event.EventNumber = eNum

		event.EventLocM = float64(parsHexValue(e[4:12])) * (math.Pow(10, -4)) * float64(d.FixedParams.FiberSpeed)

		stValue := mod(event.EventLocM, d.FixedParams.Resolution[0])
		if stValue >= d.FixedParams.Resolution[0]/2 {
			event.EventLocM = event.EventLocM + (d.FixedParams.Resolution[0] - stValue)
		} else {
			event.EventLocM = event.EventLocM + -stValue
		}

		event.EventLocM = math.Round(event.EventLocM*1000) / 1000

		event.Slope = float64(parsHexValue(e[12:16])) * 0.001
		event.SpliceLoss = float64(parsHexValue(e[16:20])) * 0.001
		if parsHexValue(e[20:28]) > 0 {
			event.RefLoss = float64((float64(parsHexValue(e[20:28])) - math.Pow(2, 32)) * 0.001)
		} else {
			event.RefLoss = float64(parsHexValue(e[20:28]))
		}

		eventType, err := hex.DecodeString(e[28:44])
		nukeIfErr(err)
		event.EventType = string(eventType)
		event.EndOfPreviousEvent = int(parsHexValue(e[44:52]))
		event.BegOfCurrentEvent = int(parsHexValue(e[52:60]))
		event.EndOfCurrentEvent = int(parsHexValue(e[60:68]))
		event.BegOfNextEvent = int(parsHexValue(e[68:76]))
		event.PeakCurrentEvent = int(parsHexValue(e[76:84]))
		if len(e) > 88 {
			if len(e) < 102 {
				comment, err := hex.DecodeString(e[84:])
				nukeIfErr(err)
				event.Comment = string(comment)
			} else {
				comment, err := hex.DecodeString(e[84:102])
				nukeIfErr(err)
				event.Comment = string(comment)
			}
		}
		d.Events[eNum] = event
	}
}

func (d *otdrRawData) export2Json() {

	var exportData = struct {
		Filename        string            `json:"File Name"`
		MiscParams      MiscParams        `json:"Misc Params"`
		FixedParams     FixInfo           `json:"Fixed Parameters"`
		TotalLoss       float64           `json:"Total Fiber Loss(dB)"`
		TotalLength     float64           `json:"Fiber Length(km)"`
		GenParams       GenParam          `json:"General Information"`
		Supplier        SupParam          `json:"Supplier Information"`
		Events          map[int]OTDREvent `json:"Key Events"`
		BellCoreVersion float64           `json:"Bellcore Version"`
	}{
		Filename:        d.Filename,
		MiscParams:      d.MiscParams,
		FixedParams:     d.FixedParams,
		TotalLoss:       d.TotalLoss,
		TotalLength:     d.TotalLength,
		GenParams:       d.GenParams,
		Supplier:        d.Supplier,
		Events:          d.Events,
		BellCoreVersion: d.BellCoreVersion,
	}

	b, err := json.MarshalIndent(exportData, "", "  ")
	nukeIfErr(err)
	_ = os.WriteFile("OTDR_Output.json", b, 0644)
	fmt.Println("Json file has been exported! - json file name: OTDR_Output.json")
}

func getCliArgs() map[string]*string {

	m := map[string]*string{}

	filePath := flag.String("file", "", "Mandatory - Path to the sor file")
	m["filePath"] = filePath

	workers := flag.String("workers", "1", "Optional - Bulk processing workers quantity. Default=1")
	m["workers"] = workers

	folderath := flag.String("folder", "", "optional - Path to the folder containing sor files")
	m["folderPath"] = folderath

	draw := flag.String("draw", "yes", "Optional - whether to draw the graph or not, yes , no. Default=yes")
	m["draw"] = draw

	json := flag.String("json", "yes", "Optional - whether to dump as json or not, yes , no. Default=yes")
	m["json"] = json

	csv := flag.String("csv", "no", "Optional - whether to dump as csv or not, yes , no. Default=no")
	m["csv"] = csv

	flag.Parse()

	if len(*m["filePath"]) == 0 {
		if len(os.Args) > 1 {
			m["filePath"] = &os.Args[1]
		}
	}

	if len(*m["filePath"]) == 0 {
		log.Fatalln("no file has been specified")
	}

	return m
}

func export2Csv(content csvFiles) {

	file, err := os.Create("csv_output.csv")

	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"Filename", "EoF"}); err != nil {
		fmt.Println("Error writing header:", err)
		return
	}
	for _, item := range content.Csvs {
		if err := writer.Write([]string{filepath.Base(item.Filename), fmt.Sprintf("%.2f", item.EOF)}); err != nil {
			fmt.Println("Error writing record:", err)
			return
		}
	}

	fmt.Println("CSV file created successfully")
}

func ParseOTDRFile(args map[string]*string) {

	var files []string
	var err error
	var wg sync.WaitGroup

	workers, _ := strconv.Atoi(*args["workers"])

	control_buffer := make(chan int, workers)

	csvContent := csvFiles{}

	if *args["folderPath"] != "" {
		files, err = getSorFilesPathFromFolder(*args["folderPath"])
		if strings.EqualFold(*args["json"], "yes") {
			fmt.Println("json export is not supported with -folder arg")
			*args["json"] = "no"
		}
		if strings.EqualFold(*args["draw"], "yes") {
			fmt.Println("drawing the graph is not supported with -folder arg")
			*args["draw"] = "no"
		}

		nukeIfErr(err)
	} else {
		files = []string{*args["filePath"]}
	}

	for _, f := range files {

		wg.Add(1)
		control_buffer <- 1

		go func(control_buffer chan int, wg *sync.WaitGroup) {
			d := ReadSorFile(f)
			fmt.Println(1)
			d.GetOrder()
			fmt.Println(2)
			d.getBellCoreVersion()
			d.getTotalLoss()
			d.getSupParams()
			d.getGenParams()
			d.getFixedParams()
			d.getDataPoints()
			d.getKeyEvents()
			d.getFiberLength()
			fmt.Println(3)

			d.getSetupParams()
			d.getMiscParams()
			d.getViewParams()
			d.getSystemParams()
			d.getAnalysisParams()
			d.getAcqParam()

			if strings.EqualFold(*args["json"], "yes") {

				d.export2Json()
			}

			if strings.EqualFold(*args["draw"], "yes") {

				d.draw()
			}

			if strings.EqualFold(*args["csv"], "yes") {
				csvContent.Csvs = append(csvContent.Csvs, csvFile{
					Filename: d.Filename,
					EOF:      d.TotalLength,
				})
			}

			wg.Done()
			<-control_buffer
		}(control_buffer, &wg)
	}

	wg.Wait()

	if strings.EqualFold(*args["csv"], "yes") {

		export2Csv(csvContent)
	}
}

func getSorFilesPathFromFolder(p string) ([]string, error) {
	l := []string{}

	err := filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".sor" || filepath.Ext(path) == ".SOR") {
			l = append(l, path)
		}
		return nil
	})

	if err != nil {
		return l, fmt.Errorf("error walking the path %v: %v", p, err)
	}

	return l, nil
}

func main() {

	// defer customPanicHandler()
	ParseOTDRFile(getCliArgs())
}
