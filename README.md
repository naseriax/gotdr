- This script will extract the supported stored information from the provided .sor file(s) which is the output of the OTDR equipment to JSON/CSV format.
- The graph generation has been added.
- In case of bulk parsing (-folder), user can specify the concurrency factor to be used to increase the processing speed:
    - Apple Macbook M3 Pro-12 CPU Core result:
        - Parsing of 3539 sor files: 1 worker : 10.3s, 8 workers: 2.25s

### Usage:
`./gotdr -file filepath -draw=yes -json=yes -csv=yes`
Or
`./gotdr -workers=10 -folder folderPath -draw=no -json=no -csv=yes`
