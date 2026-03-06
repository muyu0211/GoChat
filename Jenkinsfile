pipeline {
    agent any    // 任意节点执行

    environment {
        APP_NAME = "ginchat"
        TARGET_HOST = "192.168.74.128"
        TARGET_USER = "root"
        TARGET_PATH = "/home/muyu/下载"
    }


    stages {
        stage('Checkout') {
            steps {
                echo "===== 拉取 ${APP_NAME} 代码 ====="
                git(
                    url: 'git@github.com:muyu0211/GoChat.git',
                    credentialsId: 'server-ssh-key'
                )
            }
        }

        stage('Build in Docker') {
            agent {
                docker {
                    image 'golang:1.24.5'
                    args '-v $HOME/.cache/go-build:/root/.cache/go-build'
                }
            }
            steps {
                echo "===== 在Docker中进行构建 ====="

                sh '''
                go mod tidy
                CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${APP_NAME} ./cmd
                '''
            }
        }

        stage('Deploy') {
            steps {
                echo "===== 开始部署 ${APP_NAME} ====="

                sshagent(['server-ssh-key']) {
                    sh '''
                    scp -o StrictHostKeyChecking=no ${APP_NAME} ${TARGET_USER}@${TARGET_HOST}:${TARGET_PATH}/

                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "
                        pkill ${APP_NAME} || true
                        chmod +x ${TARGET_PATH}/${APP_NAME}
                        nohup ${TARGET_PATH}/${APP_NAME} > ${TARGET_PATH}/server.log 2>&1 &
                    "
                    '''
                }
            }
        }
    }
}
