import random
import string

N=16

s = ''.join(random.SystemRandom().choice(string.ascii_uppercase + string.digits) for _ in range(N))

base ='{"userId":"","authId":"%s","dns":["janson", "68u", "home", "company", "linksys"]}'
print base % s
