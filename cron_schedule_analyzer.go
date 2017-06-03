/*
Cron schedule analyzer script
Get the maximum simultaneous running jobs according to cron schedule and jobs average elapsed timings
Amount of simultaneously running jobs = amount of necessary CPU cores

Usage:
    ./cron_schedule_analyzer.bin <cron-schedule-file> [time-frame-days]

    Prepare plain text file with jobs cron schedule and average elapsed time in seconds
    Each row format: <cron-expression><TAB><job-average-elapsed-time-sec>
    Sample content file "my_cron_schedule":
        0 * * * *<TAB>68.5
        0 * * * *<TAB>23.0
        5 * * * *<TAB>34.2
        15 0 * * *<TAB>1356.8
    
    $ go build -o cron_schedule_analyzer.bin cron_schedule_analyzer.go
    $ ./cron_schedule_analyzer.bin my_cron_schedule
    
    time-frame-days int     - Number of days to process
                              By default time frame of 7 days is used
                              
Practical usage:
    Analyze list with 387 jobs
        $ ./cron_schedule_analyzer.bin test.cron_schedule_4 30
            Datetime                Running instances
            yyyy-01-10 08:00:00     98
    
    98 cores are need to balance servers load
    
    Move jobs with run at 0 minute to run at 0 -> 1 -> 2 -> 3 -> 4 -> 5 minute
    I.e. max displacement is 6 minutes to make minimum impact on business logic
    
    $ ./cron_schedule_analyzer.bin test.cron_schedule_5 30
        Datetime                Running instances
        yyyy-01-26 08:15:00     84
    
    Result is 14 cores less -> 2 servers with 8-core CPU can be uninstalled
*/
package main

import (
    "fmt"
    "os"
    "io/ioutil"
    "strings"
    "time"
    "strconv"
    "math"
    "sort"
    "github.com/gorhill/cronexpr"
)

const TIME_FRAME_DAYS_DEFAULT   = 7
const SEC_IN_DAY                = 86400

type RunSchema struct {
    runningJobs     int
    ts              int
    tsTime          time.Time
    tsTime1SecBack  time.Time
}

type RunSchemaListSort []RunSchema

func err(e error) {
    if (e != nil) {
        fmt.Print("Error: ")
        fmt.Print(e)
        fmt.Print("\n")
        os.Exit(1)
    }
}

func FileWriteString(fp *os.File, str string) {
    len, err := fp.WriteString(str)
    if (err != nil) {
        fmt.Println(err)
        fp.Close()
        os.Exit(1)
    } else if(len == 0) {
        fmt.Println("Error: Wrote 0 bytes to file")
        fp.Close()
        os.Exit(1)
    }
}

// Sort
func (a RunSchemaListSort) Len() int            { return len(a) }
func (a RunSchemaListSort) Swap(i, j int)       { a[i], a[j] = a[j], a[i] }
func (a RunSchemaListSort) Less(i, j int) bool  { return a[i].runningJobs > a[j].runningJobs } // Desc

func main() {        
    var e error
    
    // ------------------------------------------
    // Get input args
    var args []string = os.Args[0:]
    if (len(args) < 2 || len(args) > 3) {
        fmt.Printf("Usage: %s <cron-schedule-file> [time-frame-days]\n", args[0])
        fmt.Println("<cron-schedule-file> format: <cron-expression><TAB><job-average-elapsed-time-sec>")
        fmt.Println("<cron-schedule-file> sample: 0 * * * *\t68.5")
        fmt.Println("                             5 * * * *\t34.2")
        os.Exit(1)
    }    
    
    var cronScheduleFile string = args[1]
    
    var timeFrameSec int
    if (len(args) == 3) {        
        var timeFrameDays int
        timeFrameDays, e = strconv.Atoi(args[2])
        err(e)        
        timeFrameSec = timeFrameDays * SEC_IN_DAY        
    } else {
        timeFrameSec = TIME_FRAME_DAYS_DEFAULT * SEC_IN_DAY
    }
    // ------------------------------------------
    
    var resultFile string = "/var/tmp/cron_schedule_analyze." + strconv.Itoa(int(time.Now().Unix()))
    
    var content []byte
    content, err := ioutil.ReadFile(cronScheduleFile)
    if (err != nil) {
        fmt.Println(err)
        os.Exit(1)
    }
        
    var contentTrim string = strings.TrimSpace(string(content))
    if (contentTrim == "") {
        fmt.Println("Error: File content is empty")
        os.Exit(1)
    }
    
    type JobsData struct {
        schedule    string
        timeElapsed float64
    }
    var jobsDataList []JobsData
            
    var contentLn []string = strings.Split(contentTrim, "\n")
    for _, line := range contentLn {
        var lineTrim string = strings.TrimSpace(line)
        if (lineTrim == "") {
            continue
        }
        var cols []string = strings.Split(lineTrim, "\t")
        
        var timeElapsed float64
        timeElapsed, err := strconv.ParseFloat(cols[1], 64)
        if (err != nil) {
            fmt.Println(err)
            os.Exit(1)
        }

        var jobsData JobsData = JobsData{schedule: cols[0], timeElapsed: timeElapsed}                
        jobsDataList = append(jobsDataList, jobsData)
    }
        
    var runSchemaList []RunSchema
    
    var tsStart int = 946706400 // 2000-01-01 00:00:00
    var tsEnd int = tsStart + timeFrameSec
    
    // Prepare runSchemaList
    
    for ts := tsStart; ts < tsEnd; ts++ {
        if (ts % 60 != 0) {
            continue
        }        
        
        var tsTime time.Time = time.Unix(int64(ts), 0)
        var tsTime1SecBack time.Time = time.Unix(int64(ts - 1), 0)
        
        var runSchema RunSchema = RunSchema{
            runningJobs: 0, 
            ts: ts,
            tsTime: tsTime,
            tsTime1SecBack: tsTime1SecBack }
        
        runSchemaList = append(runSchemaList, runSchema)        
    }
    
    // Process jobs data
    
    var runSchemaListLen int = len(runSchemaList)
    for minute, runSchema := range runSchemaList {        
        if (minute % 10 == 0) {            
            fmt.Printf("Working ... %0.2f %%", (float64(minute) / float64(runSchemaListLen)) * 100)
            fmt.Print("\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b\b")
        }        
        for _, job := range jobsDataList {                                 
            var scheduleParsed *cronexpr.Expression
            scheduleParsed, err := cronexpr.Parse(job.schedule)
            if (err != nil) {
                fmt.Println(err)
                os.Exit(1)                
            }
            
            // If job run is due
            if (runSchema.tsTime == scheduleParsed.Next(runSchema.tsTime1SecBack)) {
                runSchemaList[minute].runningJobs += 1
                var runMinutesRequired int = int(math.Ceil(job.timeElapsed / 59))                
                var maxNextMinute int = minute + runMinutesRequired                                
                for nextMinute := minute + 1; nextMinute < maxNextMinute; nextMinute++ {
                    if (nextMinute <= runSchemaListLen - 1) {
                        runSchemaList[nextMinute].runningJobs += 1
                    }
                }                
            }
        }        
    }
    fmt.Println("Working ... 100.00 %")
    
    sort.Sort(RunSchemaListSort(runSchemaList))
    
    // Stdout results
    
    fmt.Println("-----------------------------------------")
    fmt.Println("Datetime\t\tRunning instances")
    for i, runSchema := range runSchemaList {
        fmt.Printf("%s\t%d\n", runSchema.tsTime.Format("yyyy-01-02 15:04:05"), runSchema.runningJobs)
        if (i > 10) {
            break
        }
    }
    fmt.Println("...")
    fmt.Println("-----------------------------------------")
        
    // Save results to file
    
    var fp *os.File
    fp, err = os.Create(resultFile)
    if (err != nil) {
        fmt.Println(err)
        os.Exit(1)
    }
    
    FileWriteString(fp, "Datetime\t\tRunning instances\n")    
    for _, runSchema := range runSchemaList {
        FileWriteString(fp, fmt.Sprintf("%s\t%d\n", runSchema.tsTime.Format("yyyy-01-02 15:04:05"), runSchema.runningJobs))
    }
    fp.Close()
    
    fmt.Printf("Complete results saved to %s\n", resultFile)
    fmt.Println("Done")
}
