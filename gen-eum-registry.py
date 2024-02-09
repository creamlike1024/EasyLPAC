import requests
from bs4 import BeautifulSoup
import json

url = 'https://euicc-manual.septs.app/docs/pki/eum/'

response = requests.get(url)
webpage = response.text
soup = BeautifulSoup(webpage, 'html.parser')

results = []

anchors = ['#ein-verified']

# 遍历 anchors
for anchor in anchors:
    h3_tag = soup.find('h3', id=anchor.strip('#'))
    if h3_tag:
        # 获取 h3 标签后的所有 li 标签
        for li in h3_tag.find_next_sibling('ul').find_all('li', recursive=False):
            results.append({
                'prefix': li.code.text.strip(),
                'manufacturer': li.find('a').text.split(' (')[0].strip(),
                'link': li.find('a')['href'],
                'country': li.code.next_sibling.strip()[1:3]
            })


json_results = json.dumps(
    results, ensure_ascii=False, indent=4)

with open('eum-registry.json', 'w') as f:
    f.write(json_results)
