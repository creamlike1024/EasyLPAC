import requests
from bs4 import BeautifulSoup
import json
from urllib.parse import urljoin
import re
import subprocess

url = 'https://euicc-manual.septs.app/docs/pki/ci/'

response = requests.get(url)
webpage = response.text
soup = BeautifulSoup(webpage, 'html.parser')

certdata_results = []

anchors = ['#gsma-root', '#gsma-test', '#sgp26',
           '#mainland-china', '#independent', '#unknown']


def extract_certificate(text):
    pattern = r'-----BEGIN CERTIFICATE-----.+?-----END CERTIFICATE-----'
    # 使用 DOTALL 标志来让'.'匹配包括换行符在内的所有字符
    # 只返回第一个结果
    match = re.search(pattern, text, re.DOTALL)
    if match:
        return match.group(0)
    else:
        return None


def get_field(output, field_name):
    for line in output.split('\n'):
        if field_name in line:
            return line.strip()
    return None


# 遍历 anchors
for anchor in anchors:
    h2_tag = soup.find('h2', id=anchor.strip('#'))
    if h2_tag:
        # 获取 h2 标签后的所有 li 标签
        for li in h2_tag.find_next_sibling('ul').find_all('li'):
            # 提取所有 keyid
            for code_tag in li.find_all('code'):
                keyid = code_tag.get_text(strip=True)
                cert_response = requests.get(
                    urljoin(url+"files/", keyid[-6:]+".txt"))
                if cert_response.status_code != 200:
                    certdata = None
                else:
                    certdata = cert_response.text

                if certdata == None:
                    cn = None
                else:
                    pem_cert = extract_certificate(certdata)
                    openssl_cmd = ['openssl', 'x509', '-text', '-noout']

                    # 启动 OpenSSL 进程
                    proc = subprocess.Popen(
                        openssl_cmd, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)

                    # 将证书内容传递给 OpenSSL 的 stdin，并获取输出
                    stdout, stderr = proc.communicate(input=pem_cert.encode())
                    # 检查是否有错误
                    if proc.returncode != 0:
                        print("OpenSSL Error:", stderr.decode())
                        exit(proc.returncode)

                    output = stdout.decode()

                    print("-----------------------------------------------------------")

                    # 获取 Subject 信息
                    subject = get_field(output, 'Subject:')
                    print(subject)

                    # # 获取 Issuer 信息
                    issuer = get_field(output, 'Issuer:')
                    print(issuer)

                    # 提取 CN
                    cn_subject_match = re.search(r'CN=([^,]+)', subject)
                    cn_issuer_match = re.search(r'CN=([^,]+)', issuer)

                    # 提取 C
                    c_subject_match = re.search(r'C=([^,]+)', subject)
                    c_issuer_match = re.search(r'C=([^,]+)', issuer)

                    # 检查证书是否为 CA 证书
                    ca_filed = get_field(
                        output, 'CA:')
                    is_ca = 'TRUE' in ca_filed if ca_filed else False
                    print('Is CA:', is_ca)
                    if is_ca:
                        if cn_issuer_match:
                            cn = cn_issuer_match.group(1)
                            print('CN:', cn)
                        else:
                            cn = None
                        if c_issuer_match:
                            c = c_issuer_match.group(1)
                            print("C:", c)
                        else:
                            c = None
                    else:
                        if cn_subject_match:
                            cn = cn_subject_match.group(1)
                            print('CN:', cn)
                        else:
                            cn = None
                        if c_subject_match:
                            c = c_subject_match.group(1)
                            print("C:", c)
                        else:
                            c = None

                certdata_results.append({
                    'C': c,
                    'CN': cn,
                    'keyID': keyid,
                    'certData': certdata
                })

json_certdata_results = json.dumps(
    certdata_results, ensure_ascii=False, indent=4)

with open('ci-registry.json', 'w') as f:
    f.write(json_certdata_results)
