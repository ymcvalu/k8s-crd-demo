自动代码生成：
```sh
# clone the code-generator repo
$ cd $GOPATH/src/k8s.io/
$ git clone git@github.com:kubernetes/code-generator.git
$ git clone git@github.com:kubernetes/gengo.git

# path of our project
$ ROOT_PACKAGE="nfs-controller"

# api version 
$ CRD_VERSION="v1"

# api group
$ CRD_GROUP="samplecrd"

# gen code
$ cd $GOPATH/src/k8s.io/code-generator
$ ./generate-groups.sh all "$ROOT_PACKAGE/pkg/client" "$ROOT_PACKAGE/pkg/apis" "$CRD_GROUP:$CRD_VERSION"
```

如果出现错误：
```
generate-groups.sh: line 56: `codegen::join': not a valid identifier
```
替换`generate-groups.sh`中的函数名`codegen::join`为`codegen_join`