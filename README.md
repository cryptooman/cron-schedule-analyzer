# cron-schedule-analyzer
Cron schedule analyzer script

<pre>
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
    
    $ mkdir cronexpr
    $ export GOPATH=`pwd`/cronexpr
    $ go get github.com/gorhill/cronexpr
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
</pre>