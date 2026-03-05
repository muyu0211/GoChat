pipeline {
    agent any    // 任意节点执行

    environment {
        APP_NAME = 'ginchat'
        GO_HOME = '/usr/local/go'       // go 地址
    }

    stages {
        stage('Debug') {
            steps {
                sh 'docker ps -a'
            }
        }
        // stage('Checkout') {
        //     // 1. 拉取代码
        //     steps {
        //         checkout scm
        //     }
        // }

        // stage('Build in Docker') {
        //     agent {
        //         docker {
        //             image 'golang:1.24.5'
        //             args '-v $HOME/.cache/go-build:/root/.cache/go-build'
        //         }
        //     }
        //     steps {
        //         sh '''
        //         go mod tidy
        //         CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gochat
        //         '''
        //     }
        // }

        // stage('Deploy') {
        //     // 3. 将二进制文件发送到宿主机，并执行重启脚本
        //     steps {

        //     }
        // }
    }
}
