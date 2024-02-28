#!/usr/bin/env python3
import os.path
import re
from urllib.parse import urljoin
from zipfile import ZipFile, ZIP_DEFLATED

import requests

BASE_PATH = "https://euicc-manual.septs.app/docs/pki/ci/"
RE_FILE_NAME = re.compile(r"files/[\da-f]{6}\.txt")


def main():
    response = requests.get(BASE_PATH)
    response.raise_for_status()
    with ZipFile("ci-registry.zip", "w", compression=ZIP_DEFLATED, compresslevel=9) as bundle:
        print("ci-registry.zip creating")
        for file_path in RE_FILE_NAME.findall(response.text):
            with bundle.open(os.path.basename(file_path), "w") as fp:
                resp = requests.get(urljoin(BASE_PATH, file_path))
                resp.raise_for_status()
                print(resp.url, "downloaded")
                fp.write(resp.content)
        print("ci-registry.zip created")


if __name__ == "__main__":
    main()
