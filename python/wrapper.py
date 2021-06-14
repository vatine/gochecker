#! /usr/bin/python3

import json
import logging
import os
import re
import requests
import subprocess
import sys


BIN_IMPORT_RE = re.compile(
    r'^/go/pkg/mod.*[.]go:[1-9][0-9]*:[0-9]+: .* is a program, not'
    )
VET_TYPE_RE = re.compile(
    r'cannot use .* as type .* in argument'
    )

def unused_download(pkg, version):
    """
    Download a module.
    """

    logging.debug("About to go get -tu %s@%s", pkg, version)
    proc = subprocess.run(['go', 'get', '-tu', pkg_and_version(pkg, version)], cwd='/go/testmod')
    logging.debug("Status is %d", proc.returncode)

    return proc.returncode == 0

def go_fmt(path):
    """
    Do a 'go fmt' check, retrun True if nothing needed changing, False if something did.
    """
    logging.debug("About to gofmt %s", path)
    proc = subprocess.run(['gofmt', '-d', path], stdout=subprocess.PIPE)
    return len(proc.stdout) == 0


def introspect(pkg):
    """
    Get a list of dicts, each specifying a package in the module
    """
    logging.debug("About to introspect %s", pkg)
    proc = subprocess.run(['go', 'list', '-json', f'{pkg}/...'], cwd=pkg_cwd('buildmod', 'ignore'), capture_output=True)
    if proc.returncode != 0:
        return []

    return parse_multi_json(proc.stdout.decode('utf-8'))


def parse_multi_json(in_data):
    logging.debug("Parsing json")
    output = []
    decoder = json.JSONDecoder()
    while len(in_data) > 1:
        obj, length = decoder.raw_decode(in_data)
        output.append(obj)
        in_data = in_data[length:].lstrip()

    logging.debug("Found %d items", len(output))
    return output


def pkg_and_version(pkg, version):
    return f'{pkg}@{version}'


def go_escape(c):
    if c.isupper():
        return '!' + c.lower()
    return c


def go_fmt_check(target):
    """
    Run gofmt on all the files in a build target. Return True, if all files are
    OK, and False otherwise.
    """
    dir = target['Dir']
    result = True
    for name in target['GoFiles']:
        result = result and go_fmt(os.path.join(dir, name))
        if not result:
            return False
    return result


def pkg_cwd(pkg, version):
    if pkg == 'buildmod':
        return '/tmp/buildmod'

    escaped = ''.join([go_escape(c) for c in pkg_and_version(pkg, versionn)])
    return os.path.join('/go/pkg/mod', escaped)


def prepare_build():
    build_dir = pkg_cwd('buildmod', 'ignore')
    if not os.path.exists(build_dir):
        os.mkdir(build_dir)

    subprocess.run('go mod init buildmod'.split(), cwd=build_dir)


def download(pkg, version):
    logging.debug("Preparing to download %s @ %s", pkg, version)
    prepare_build()
    build_dir = pkg_cwd('buildmod', 'ignore')
    pkg_designator = pkg_and_version(pkg, version)

    logging.debug("  Adding as a requirement.")
    logging.debug("  In %s", build_dir)
    subprocess.run(['go', 'mod', 'edit', '-require', pkg_designator], cwd=build_dir)
    logging.debug("  Downloading...")
    proc = subprocess.run(['go', 'get', '-t', '-u', pkg_designator], cwd=build_dir , stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

    out = proc.stdout.decode('utf-8')
    logging.debug("  output is %s", out)
    logging.debug("  exit status is %d", proc.returncode)
    if proc.returncode == 0:
        return True, True
    else:
        if "build constraints exclude all Go files" in out:
            return True, False
        if BIN_IMPORT_RE.search(out):
            return True, False
        if VET_TYPE_RE.search(out):
            return True, False
        return False, False


def go(operation, pkg):
    logging.debug("Running go %s %s", operation, pkg)
    build_dir = pkg_cwd('buildmod', 'ignore')
    proc = subprocess.run(['go', operation, pkg], cwd=build_dir)
    return proc.returncode == 0


def test_and_build(pkg, version):
    output = {}
    
    output['downloadSucceeded'], cont = download(pkg, version)
    if not output['downloadSucceeded']:
        logging.info("Download failed, exiting early...")
        return output

    all_targets = []
    buildable_targets = 0
    testable_targets = 0
    all_builds_pass = True
    all_tests_pass = True
    failed_builds = []
    failed_tests = []
    vet_passed = []
    failed_vets = []
    fmt_failed = []
    if not cont:
        logging.info("Download succeeded, nothing to build.")
        return output
    all_targets = introspect(pkg)


    for target in all_targets:
        if len(target.get('GoFiles', [])):
            logging.debug("  Checking gofmt %s", target['ImportPath'])
            if not go_fmt_check(target):
                fmt_failed.append(target)
            logging.debug("  Building go target %s", target['ImportPath'])
            buildable_targets += 1
            if go('build', target['ImportPath']):
                logging.debug("    Success building %s", target['ImportPath'])
            else:
                all_builds_pass = False
                logging.debug("    Build of %s failed", target['ImportPath'])
                failed_builds.append(target['ImportPath'])
            if go('vet', target['ImportPath']):
                vet_passed.append(target['ImportPath'])
            else:
                failed_vets.append(target['ImportPath'])

        if len(target.get('TestGoFiles', [])):
            logging.debug("  Testing go target %s", target['ImportPath'])
            testable_targets += 1
            if go('test', target['ImportPath']):
                pass
            else:
                all_tests_pass = False
                failed_tests.append(target['ImportPath'])

    output['buildableTargets'] = buildable_targets
    output['allBuildsPass'] = all_builds_pass
    output['testableTargets'] = testable_targets
    output['allTestsPass'] = all_tests_pass
    output['failedBuilds'] = failed_builds
    output['failedTests'] = failed_tests
    output['passedVets'] = vet_passed
    output['failedVets'] = failed_vets
    output['failedFmt'] = fmt_failed

    return output


def send_report(url, pkg, version, data):
    payload = { 'package': pkg_and_version(pkg, version),
                'data': data
    }

    requests.post(url, json=payload)


def process(pkg, version, url):
    data = test_and_build(pkg, version)
    send_report(url, pkg, version, data)


def main():
    logging.basicConfig(level=logging.DEBUG)
    pkg, version, url = sys.argv[1:]
    process(pkg, version, url)


if __name__ == '__main__':
    main()
