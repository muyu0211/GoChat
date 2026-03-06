pipeline {
    agent any    // 任意节点执行

    environment {
        APP_NAME = "ginchat"
        TARGET_HOST = "192.168.74.128"
        TARGET_USER = "root"
        TARGET_PATH = "/home/muyu/deploy"
    }

    stages {
        stage('Checkout') {
            steps {
                echo "===== 拉取 ${APP_NAME} 代码 ====="
                cleanWs() // 确保在拉取代码前，工作区是绝对干净的
                git(
                    branch: 'master',
                    url: 'git@github.com:muyu0211/GoChat.git',
                    credentialsId: 'server-ssh-key'
                )
                sh 'git log -1'
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
                sh '''
                echo "===== 在Docker中进行构建 ====="
                go clean -cache
                go mod tidy
                CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${APP_NAME} ./cmd
                md5sum ${APP_NAME}
                '''
            }
        }

        stage('Deploy') {
            steps {
                echo "===== 开始部署 ${APP_NAME} ====="

                sshagent(['server-ssh-key']) {
                    sh '''
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "pkill ${APP_NAME} || true"
                    
                    sleep 2

                    # 3. 使用 scp 覆盖文件了
                    scp -o StrictHostKeyChecking=no ${APP_NAME} ${TARGET_USER}@${TARGET_HOST}:${TARGET_PATH}/

                     # 4. 远程执行启动命令
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "
                        chmod +x ${TARGET_PATH}/${APP_NAME}
                        cd ${TARGET_PATH}
                        echo "===== 启动 ${APP_NAME} ====="
                        pwd
                        nohup ./${APP_NAME} > server.log 2>&1 &
                    "
                    '''
                }
            }
        }
    }
}
