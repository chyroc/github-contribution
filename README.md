# github-contribution
list self github contribution / 开源项目贡献统计

# install
```
go get -u github.com/chyroc/github-contribution
```

# use

## github contribution
```
github-contribution -t <github token> [-v]
```

## side project
```
github-contribution -t <github token> -c config.json [-v]
```

config.json:
```
{
  "side_project": [
    {
      "name": "WechatSogou",
      "introduction": "基于搜狗微信搜索的微信公众号爬虫接口",
      "url": "https://github.com/Chyroc/WechatSogou"
    }
  ]
}
```

example repo: [Chyroc/contribute](https://github.com/Chyroc/contribute)