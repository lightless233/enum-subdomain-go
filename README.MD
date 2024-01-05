# enum-subdomain-go

## 编译构建
```shell
go build -o enum-subdomain-go
```

## 使用方法
```shell
./enum-subdomain-go -t <target> -x <D,L,F> -d <dict_file> -l <brute_length> -f <fofa_token> -o <output_file>
# 例如
./enum-subdomain-go -t baidu.com -x dlf -d my_dict.txt -l 1-3 -f fofa_email|fofa_token -o out.txt
```