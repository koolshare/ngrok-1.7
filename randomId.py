#!/bin/python env
# -*- coding: UTF-8 -*-
import random
import string

N=16

s = ''.join(random.SystemRandom().choice(string.ascii_uppercase + string.digits) for _ in range(N))

txt = '''
服务器：etunnel.net
端口： 4443
用户名： %s
密码： %s
可使用的子域名：%s
访问方式：
http://子域名.etunnel.net:8080
https://子域名.etunnel.net
访问比如使用 http://%s.etunnel.net:8080
'''

#s2 = ['"' + s4 + '"' for s4 in 'web api jsr jsn jsv'.split()]
s2 = "route pi nas iio zeus".split()
rlt = str(s2).replace("'", '"')
print '{"userId":"%s","authId":"%s","dns":%s}' % (s2[0], s, rlt)
print txt % (s, s2[0], rlt, s2[1])
