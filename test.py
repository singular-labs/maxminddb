#!/usr/local/bin/python
import json
import maxminddb

ref = maxminddb.open_database('GeoLite2-City.mmdb')
sng = maxminddb.open_database('Singular.mmdb')
x = ref.get('70.114.203.247')
print '\033[1;31m%s\033[0m' % 'GeoLite2-City.mmdb'
print '-' * 72
print json.dumps(x, indent=2, ensure_ascii=False)
print
x = sng.get('70.114.203.247')
print '\033[1;31m%s\033[0m' % 'Singular.mmdb'
print '-' * 72
print json.dumps(x, indent=2, ensure_ascii=False)
