# MaxMindDB writer library for Go

This is a very basic Go library to write MaxMindDB files from Go:

* it only supports IPv4
* the only locale language is English
* the data model is simplistic, and supports only country, a single region, city and latitude/longitude

It was born out of frustration with the official MaxMindDB writer library written in Perl. A side benefit is that the files are more compact than the official `.mmdb` files from MaxMind, as they do not include all localized variants of place names and other seldom-used information. The GeoIP2-City database shrinks from 124MB to 76MB, for instance.

here is the MaxMind GeoIP2-City data for the IP `96.74.71.17`
```
{
  "country": {
    "geoname_id": 6252001, 
    "iso_code": "US", 
    "names": {
      "ru": "США", 
      "fr": "États-Unis", 
      "en": "United States", 
      "de": "USA", 
      "zh-CN": "美国", 
      "pt-BR": "Estados Unidos", 
      "ja": "アメリカ合衆国", 
      "es": "Estados Unidos"
    }
  }, 
  "registered_country": {
    "geoname_id": 6252001, 
    "iso_code": "US", 
    "names": {
      "ru": "США", 
      "fr": "États-Unis", 
      "en": "United States", 
      "de": "USA", 
      "zh-CN": "美国", 
      "pt-BR": "Estados Unidos", 
      "ja": "アメリカ合衆国", 
      "es": "Estados Unidos"
    }
  }, 
  "continent": {
    "geoname_id": 6255149, 
    "code": "NA", 
    "names": {
      "ru": "Северная Америка", 
      "fr": "Amérique du Nord", 
      "en": "North America", 
      "de": "Nordamerika", 
      "zh-CN": "北美洲", 
      "pt-BR": "América do Norte", 
      "ja": "北アメリカ", 
      "es": "Norteamérica"
    }
  }, 
  "location": {
    "latitude": 37.751, 
    "accuracy_radius": 1000, 
    "longitude": -97.822
  }
}
```
and as reduced by this library:
```
{
  "country": {
    "iso_code": "US", 
    "names": {
      "en": "United States"
    }
  }, 
  "subdivisions": [
    {
      "names": {
        "en": "California"
      }
    }
  ], 
  "location": {
    "latitude": 37.76969909667969, 
    "longitude": -122.39330291748047
  }, 
  "city": {
    "names": {
      "en": "San Francisco"
    }
  }
}
```


## Building

You can build the library and its test program using:

    go get -f -t -u -v github.com/singular-labs/maxminddb/...

