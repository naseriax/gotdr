package main

import "time"

type otdrRawData struct {
	Filename        string `json:"File Name"`
	Decodedfile     string
	HexData         string
	SecLocs         map[string][]int
	FixedParams     FixInfo           `json:"Fixed Parameters"`
	TotalLoss       float64           `json:"Total Fiber Loss(dB)"`
	TotalLength     float64           `json:"Fiber Length(km)"`
	GenParams       GenParam          `json:"General Information"`
	Supplier        SupParam          `json:"Supplier Information"`
	Events          map[int]OTDREvent `json:"Key Events"`
	BellCoreVersion float64           `json:"Bellcore Version"`
	DataPoints      [][]float64
	Distance        []float64
	Power           []float64
	MiscParams      MiscParams
}

type MiscParams struct {
	Mode      string `json:"Scan Mode"`
	FiberType string `json:"Fiber Type"`
}

// SupParam is the Supplier Parameters extracted from the sor file.
type SupParam struct {
	OTDRSupplier   string `json:"OTDR Supplier"`
	OTDRName       string `json:"OTDR Name"`
	OTDRsn         string `json:"OTDR SN"`
	OTDRModuleName string `json:"OTDR Module Name"`
	OTDRModuleSN   string `json:"OTDR Module SN"`
	OTDRswVersion  string `json:"OTDR SW Version"`
	OTDROtherInfo  string `json:"OTDR Other Info"`
}

// GenParams is the General Parameters extracted from the sor file.
type GenParam struct {
	CableID        string `json:"Cable Id"`
	Lang           string `json:"Language"`
	FiberID        string `json:"Fiber Id"`
	LocationA      string `json:"Location A"`
	LocationB      string `json:"Location B"`
	BuildCondition string `json:"Build Condition"`
	Comment        string `json:"Comment"`
	CableCode      string `json:"Cable Code"`
	Operator       string `json:"Operator"`
	FiberType      string `json:"Fiber Type"`
	OTDRWavelength string `json:"OTDR Wavelength"`
}

// OTDREvent is the event information extracted from the sor file.
type OTDREvent struct {
	EventType          string  `json:"Event Type"`
	EventLocM          float64 `json:"Event Point(m)"`
	EventNumber        int     `json:"Event Number"`
	Slope              float64 `json:"Slope(dB)"`
	SpliceLoss         float64 `json:"Splice Loss(dB)"`
	RefLoss            float64 `json:"Reflection Loss(dB)"`
	EndOfPreviousEvent int     `json:"Previous Event-End"`
	BegOfCurrentEvent  int     `json:"Current Event-Start"`
	EndOfCurrentEvent  int     `json:"Current Event-End"`
	BegOfNextEvent     int     `json:"Next Event-Start"`
	PeakCurrentEvent   int     `json:"Peak point"`
	Comment            string  `json:"Comment"`
	Power              float64 `json:"Power"`
}

// FixInfos struct is the Fixed parameters extracted from the sor file.
type FixInfo struct {
	DateTime       time.Time
	Unit           string
	ActualWL       float64   `json:"Actual Wavelength"`
	PulseWidthNo   int64     `json:"Pulse Width No"`
	PulseWidth     []int64   `json:"Pulse Width(ns)"`
	SampleQTY      []int64   `json:"Sample Quantity"`
	IOR            float64   `json:"IOR"`
	RefIndex       float64   `json:"Refraction Index"`
	FiberSpeed     float64   `json:"Fiber Light Speed"`
	Resolution     []float64 `json:"Scan Resolution"`
	Backscattering float64   `json:"Back-Scattering"`
	Averaging      int64     `json:"Averaging"`
	AveragingTime  float64   `json:"Averaging Time"`
	Range          []float64 `json:"Scan Range"`
	AO             float64   `json:"AO"`
	AOD            float64   `json:"AOD"`
}
