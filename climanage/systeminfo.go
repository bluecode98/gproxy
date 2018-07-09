package climanage

import (
	"os"
	"encoding/csv"
	"fmt"
)

func SystemInfoString(serverId string) ([]string, error) {
	infoFilename := fmt.Sprintf(".\\data\\%s\\systeminfo.csv", serverId)
	infoFile, err := os.Open(infoFilename)
	defer infoFile.Close()
	if err != nil {
		return nil, err
	}

	// read csv
	reader := csv.NewReader(infoFile)
	serverInfo, _ := reader.ReadAll()
	return serverInfo[1], nil

}
