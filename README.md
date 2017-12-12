# MaxMindDB writer library for Go

This is a very basic Go library to write MaxMindDB files from Go:

* it only supports IPv4
* the data model is simplistic, and supports only country, a single region, city and latitude/longitude

It was born out of frustration with the official MaxMindDB writer library written in Perl. A side benefit is that the files are more compact than the official `.mmdb` files from MaxMind, as they do not include all localized variants of place names and other seldom-used information. The GeoIP2-City database shrinks from 124MB to 76MB, for instance.

## Building

You can build the library and its test program using:

    go get -f -t -u -v github.com/singular-labs/maxminddb/...
