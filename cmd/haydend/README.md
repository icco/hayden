# HaydenD

HaydenD is a webserver for the hayden library. It focuses on keeping local copies of all data it asks other archive services and does periodic rescrapes.

## Database

requests

 - url : string
 - links : json
 - timestamp

scrape_history

 - url : string
 - content : text
 - timestamp
 - screenshot : text
