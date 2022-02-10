#! /usr(/bin/python3

import json
import sys


def load_data(filename):
    with open(filename) as in_file:
        return json.load(in_file)
        
    
def rescan(data):
    for pkg in sorted(data):
        if not data[pkg]['downloadSucceeded']:
            mod, version = pkg.split('@')
            print(f"docker run --rm --env-file /tmp/go_data/env gobuilder:manual {mod} {version} http://192.168.1.2:8080/api/report")
            print("sleep 30")

def rebuild(data):
    for name in sorted(data):
        pkg = data[name]
        if pkg['downloadSucceeded'] and not pkg['allBuildsPass']:
            mod, version = name.split('@')
            print(f"# {name}")
            print(f"docker run --rm --env-file /tmp/go_data/env gobuilder:manual {mod} {version} http://192.168.1.2:8080/api/report")
            print("sleep 30")
            
def main(args):
    func = {'rescan': rescan, 'rebuild': rebuild}.get(args[0], None)
    if func:
        filename = args[1]
        data = load_data(filename)
        func(data)
    else:
        print(f'Unknown sub-command {args[0]}')

    
if __name__ == '__main__':
    main(sys.argv[1:])
