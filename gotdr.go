/*
Author: Naseredin Aramnejad naseredin.aramnejad@gmail.com
This script is designed to extract all the possible information from the
given sor file.
each sor file (Provided by OTDR Equipment) contains multiple data blocks
and since it's a binary file, it should be red per byte.

Formulas and blueprint of this script are inspired by the information provided by:
Sidney Li
http://morethanfootnotes.blogspot.com/2015/07/the-OTDR-optical-time-domain.html
*/
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

//lightSpeed is the speed of light in a vaccuum to be used for refractive index and fiber length calculation.
const lightSpeed = 299.79181901 // m/µsec

//OTDRData is the struct wrapping all the extracted information and will be exported as JSON.
type OTDRData struct {
	Supplier        SupParam
	GenInfo         GenParam `json:"General Information"`
	Events          []OTDREvent
	FixInfo         FixInfo `json:"Fixed Parameters"`
	FiberLength     float32 `json:"Fiber Length(km)"`
	BellCoreVersion float32 `json:"Bellcore Version"`
	TotalLoss       float32 `json:"Total Fiber Loss(dB)"`
	AvgLoss         float32 `json:"Average Loss per Km(dB)"`
}

//SupParam is the Supplier Parameters extracted from the sor file.
type SupParam struct {
	OTDRSupplier   string `json:"OTDR Supplier"`
	OTDRName       string `json:"OTDR Name"`
	OTDRsn         string `json:"OTDR SN"`
	OTDRModuleName string `json:"OTDR Module Name"`
	OTDRModuleSN   string `json:"OTDR Module SN"`
	OTDRswVersion  string `json:"OTDR SW Version"`
	OTDROtherInfo  string `json:"OTDR Other Info"`
}

//GenParams is the General Parameters extracted from the sor file.
type GenParam struct {
	CableID        string `json:"Cable Id"`
	FiberID        string `json:"Fiber Id"`
	LocationA      string
	LocationB      string
	BuildCondition string `json:"Build Condition"`
	Comment        string
	CableCode      string `json:"Cable Code"`
	Operator       string
	FiberType      string `json:"Fiber Type"`
	OTDRWavelength string `json:"OTDR Wavelength"`
}

//OTDREvent is the event information extracted from the sor file.
type OTDREvent struct {
	EventType          string  `json:"Event Type"`
	EventLocM          float32 `json:"Event Point(m)"`
	EventNumber        int     `json:"Event Number"`
	Slope              float32 `json:"Slope(dB)"`
	SpliceLoss         float32 `json:"Splice Loss(dB)"`
	RefLoss            float32 `json:"Reflection Loss(dB)"`
	EndOfPreviousEvent int     `json:"Previous Event-End"`
	BegOfCurrentEvent  int     `json:"Current Event-Start"`
	EndOfCurrentEvent  int     `json:"Current Event-End"`
	BegOfNextEvent     int     `json:"Next Event-Start"`
	PeakCurrentEvent   int     `json:"Peak point"`
}

//FixInfos struct is the Fixed parameters extracted from the sor file.
type FixInfo struct {
	DateTime       time.Time
	Unit           string
	ActualWL       float32 `json:"Actual Wavelength"`
	PulseWidthNo   int64   `json:"Pulse Width No"`
	PulseWidth     int64   `json:"Pulse Width(ns)"`
	SampleQTY      int64   `json:"Sample Quantity"`
	IOR            int64
	RefIndex       float32 `json:"Refraction Index"`
	FiberSpeed     float32 `json:"Fiber Light Speed"`
	Resolution     float32 `json:"Scan Resolution"`
	Backscattering float32 `json:"Back-Scattering"`
	Averaging      int64
	AveragingTime  float32 `json:"Averaging Time"`
	Range          float32 `json:"Scan Range"`
}

//errDealer will handle the errors.
func errDealer(err error) {
	if err != nil {
		panic(err)
	}
}

/*
	ReadSorFile function opens the sor file and returns a hex string (hexData)
	and a text string (charString) from the file to the main function,
	Basically reading the whole file and putting it in RAM.
*/
func ReadSorFile(filename string) (string, string) {

	var array []byte
	var hexData string

	f, err := os.Open(filename)
	errDealer(err)

	defer f.Close()

	for {
		eachByte := make([]byte, 1)
		_, err = f.Read(eachByte)

		if err == io.EOF {
			break
		}
		errDealer(err)

		array = append(array, eachByte...)
	}
	//Converting the byte array into a hex String
	hexData = hex.EncodeToString(array)

	//Converting the HexData to a text string
	chars, err := hex.DecodeString(hexData)
	errDealer(err)

	charString := string(chars)
	return hexData, charString
}

//Reverse will reverse the hex string in every 2 bytes. Example: 0ABCD123 => 23D1BC0A.
func Reverse(s string) string {
	str := ""
	for ind := 0; ind < len(s); ind += 2 {
		str = s[ind:ind+2] + str
	}
	return str
}

//HexParser calls the Reverse() funcition to reverse the order of the provided HexString and then converts it's value to int64.
func HexParser(hexData string) int64 {
	output, err := strconv.ParseInt(Reverse(hexData), 16, 64)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return output
}

//FixedParams function extracts the Fixed Parameters from the sor file and stores it in FixInfos struct.
func FixedParams(hexData, charString string) FixInfo {

	fixInfo := hexData[(strings.Index(charString[224:], "FxdParams")+234)*2 : (strings.Index(charString[224:], "DataPts")+224)*2]

	unit, err := hex.DecodeString(fixInfo[8:12])
	errDealer(err)

	ior := HexParser(fixInfo[56:64])
	refIndex := float32(ior) * float32(math.Pow(10, -5))
	fiberSpeed := lightSpeed / refIndex
	resolution := float32(HexParser(fixInfo[40:48])) * float32(math.Pow(10, -8)) * fiberSpeed
	sampleQTY := HexParser(fixInfo[48:56])

	fixParam := FixInfo{
		DateTime:       time.Unix(HexParser(fixInfo[:8]), 0),
		Unit:           string(unit),
		ActualWL:       float32(HexParser(fixInfo[12:16])) / 10.0,
		PulseWidthNo:   HexParser(fixInfo[32:36]),
		PulseWidth:     HexParser(fixInfo[36:40]),
		SampleQTY:      sampleQTY,
		IOR:            ior,
		RefIndex:       refIndex,
		FiberSpeed:     fiberSpeed,
		Resolution:     resolution,
		Backscattering: float32(HexParser(fixInfo[64:68])) * -0.1,
		Averaging:      HexParser(fixInfo[68:76]),
		AveragingTime:  float32(HexParser(fixInfo[76:80])) / 600,
		Range:          float32(sampleQTY) * resolution,
	}
	return fixParam
}

//SupParams function extracts the Supplier Parameters from the sor file and stores it in SupParam struct.
func SupParams(charString string) SupParam {
	supString := charString[strings.Index(charString[224:], "SupParams")+234 : strings.Index(charString[224:], "FxdParams")+224]
	slicedParams := strings.Split(supString, "\x00")[:7]

	supInfo := SupParam{
		OTDRSupplier:   strings.TrimSpace(slicedParams[0]),
		OTDRName:       strings.TrimSpace(slicedParams[1]),
		OTDRsn:         strings.TrimSpace(slicedParams[2]),
		OTDRModuleName: strings.TrimSpace(slicedParams[3]),
		OTDRModuleSN:   strings.TrimSpace(slicedParams[4]),
		OTDRswVersion:  strings.TrimSpace(slicedParams[5]),
		OTDROtherInfo:  strings.TrimSpace(slicedParams[6]),
	}
	return supInfo
}

//GenParams function extracts the General Parameters from the sor file and stores it in GenParam struct.
func GenParams(charString string) GenParam {

	genString := charString[strings.Index(charString[224:], "GenParams")+234 : strings.Index(charString[224:], "SupParams")+224]
	slicedParams := strings.Split(genString, "\x00")

	genInfo := GenParam{
		CableID:        strings.TrimSpace(slicedParams[0]),
		FiberID:        strings.TrimSpace(slicedParams[1]),
		LocationA:      strings.TrimSpace(slicedParams[2][4:]),
		LocationB:      strings.TrimSpace(slicedParams[3]),
		CableCode:      strings.TrimSpace(slicedParams[4]),
		BuildCondition: strings.TrimSpace(slicedParams[5]),
		Operator:       strings.TrimSpace(slicedParams[13]),
		Comment:        strings.TrimSpace(slicedParams[14]),
		FiberType:      "G." + strconv.FormatInt(HexParser(hex.EncodeToString([]byte(slicedParams[2][:2]))), 10),
		OTDRWavelength: strconv.FormatInt(HexParser(hex.EncodeToString([]byte(slicedParams[2][2:4]))), 10) + " nm",
	}
	return genInfo
}

//FiberLength calculates the fiber length and returns it.
func FiberLength(hexData, charString string, fixParams FixInfo) float32 {
	length := float32(HexParser(hexData[(strings.Index(charString[224:], "WaveMTSParams")+210)*2 : (strings.Index(charString[224:], "WaveMTSParams")+214)*2]))
	return (length * float32(math.Pow(10, -4)) * lightSpeed / fixParams.RefIndex) / 1000
}

//BellCoreVersion reads the bellcore version from the sor file and returns it.
func BellCoreVersion(hexData, charString string) float32 {
	return float32(HexParser(hexData[(strings.Index(charString, "Map")+4)*2:(strings.Index(charString, "Map")+5)*2])) / 100.0
}

//TotalLoss reads the total loss of the fiber from the sor file and returns it.
func TotalLoss(hexData, charString string) float32 {
	totallossinfo := hexData[(strings.Index(charString[224:], "WaveMTSParams")+202)*2 : (strings.Index(charString[224:], "WaveMTSParams")+206)*2]
	return float32(HexParser(totallossinfo)) * 0.001
}

//KeyEvents function extracts the events information from the sor file and stores it in OTDREvent struct.
func KeyEvents(hexData string, charString string, fiberSpeed float32, resolution float32) []OTDREvent {
	var keyevents []OTDREvent
	events := hexData[(strings.Index(charString[224:], "KeyEvents")+224)*2 : (strings.Index(charString[224:], "WaveMTSParams")+224)*2]
	evnumbers := HexParser(events[20:24])
	var eventHEX = []string{}
	for ind := 0; ind < int(88*evnumbers); ind += 88 {
		eventHEX = append(eventHEX, events[24:][ind:ind+88])
	}

	for e := range eventHEX {
		eventType, err := hex.DecodeString(eventHEX[e][28:44])
		errDealer(err)

		EventLocM := float32(HexParser(eventHEX[e][4:12])) * float32(math.Pow(10, -4)) * fiberSpeed
		stValue := float32(math.Mod(float64(EventLocM), float64(resolution)))
		if stValue >= (resolution / 2.0) {
			EventLocM = EventLocM + resolution - stValue
		} else {
			EventLocM = EventLocM - stValue
		}

		var refLoss float32
		if HexParser(eventHEX[e][20:28]) > 0 {
			refLoss = float32((float64(HexParser(eventHEX[e][20:28])) - math.Pow(2, 32)) * 0.001)
		} else {
			refLoss = float32(HexParser(eventHEX[e][20:28]))
		}

		eventEntry := OTDREvent{
			EventNumber:        int(HexParser(eventHEX[e][:4])),
			EventLocM:          EventLocM,
			Slope:              float32(HexParser(eventHEX[e][12:16])) * 0.001,
			SpliceLoss:         float32(HexParser(eventHEX[e][16:20])) * 0.001,
			RefLoss:            refLoss,
			EventType:          string(eventType),
			EndOfPreviousEvent: int(HexParser(eventHEX[e][44:52])),
			BegOfCurrentEvent:  int(HexParser(eventHEX[e][52:60])),
			EndOfCurrentEvent:  int(HexParser(eventHEX[e][60:68])),
			BegOfNextEvent:     int(HexParser(eventHEX[e][68:76])),
			PeakCurrentEvent:   int(HexParser(eventHEX[e][76:84])),
		}

		keyevents = append(keyevents, eventEntry)
		if string(eventEntry.EventType[1]) == "E" {
			break
		}
	}
	return keyevents
}

func JSONExport(data OTDRData) {
	b, err := json.MarshalIndent(data, "", "  ")
	errDealer(err)
	_ = ioutil.WriteFile("OTDR_Output.json", b, 0644)
	fmt.Println("Json file has been exported! - json file name: OTDR_Output.json")
}

func main() {

	//This part is for reading the Sor file by getting it's path from the user.
	var sorFileName string
	fmt.Print("Enter the .sor filename: ")
	fmt.Scanln(&sorFileName)

	/*
		Invoking the filereader function, the result will be
		a hex version of the sor file (hexData)and a text encoded
		version of the sor file (charString)hexData will be
		used to extract the numeric values while charString is used
		to find the related sections and also for text data extraction.
	*/
	hexData, charString := ReadSorFile(sorFileName)

	fixed := FixedParams(hexData, charString)
	loss := TotalLoss(hexData, charString)
	length := FiberLength(hexData, charString, fixed)

	// OTDRData is the main struct which contains and gather all the extracted information to be converted into JSON format.
	OTDRExtractedData := OTDRData{
		FixInfo:         fixed,                                                              //Fixed params that are extracted above.
		Supplier:        SupParams(charString),                                              //OTDR Module Supplier Information.
		GenInfo:         GenParams(charString),                                              //OTDR scan General Information.
		Events:          KeyEvents(hexData, charString, fixed.FiberSpeed, fixed.Resolution), //OTDR scan key events like loss events and reflection events.
		TotalLoss:       loss,                                                               //Total fiber loss value of the scan, end to end in dB.
		FiberLength:     length,                                                             //Total fiber length, end to end.
		BellCoreVersion: BellCoreVersion(hexData, charString),                               //Bellcore version of the file, 2.1 is supported by this script.
		AvgLoss:         loss / length,
	}

	JSONExport(OTDRExtractedData)
}
