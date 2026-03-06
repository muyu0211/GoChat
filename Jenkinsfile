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
                go clean -cache    # 强制清理构建缓存
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
                    # 1. 杀进程
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "pkill ${APP_NAME} || true"
                    sleep 2

                    # 2. 删除旧文件 (确保覆盖)
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "rm -f ${TARGET_PATH}/${APP_NAME}"

                    # 3. 传输新文件
                    scp -o StrictHostKeyChecking=no ${APP_NAME} ${TARGET_USER}@${TARGET_HOST}:${TARGET_PATH}/
                    
                    # 4. 验证是否传输成功 (可选)
                    # 比较本地 build 出来的 ginchat 和 服务器上的 ginchat 的 md5
                    echo "Checking integrity..."
                    LOCAL_MD5=$(md5sum ${APP_NAME} | awk '{print $1}')
                    REMOTE_MD5=$(ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "md5sum ${TARGET_PATH}/${APP_NAME}" | awk '{print $1}')
                    
                    if [ "$LOCAL_MD5" != "$REMOTE_MD5" ]; then
                        echo "Error: MD5 mismatch! Deploy failed."
                        exit 1
                    fi

                    # 5. 启动
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "
                        chmod +x ${TARGET_PATH}/${APP_NAME}
                        cd ${TARGET_PATH}
                        nohup ./${APP_NAME} > server.log 2>&1 &
                    "
                    '''
                }
            }
        }
    }
}
