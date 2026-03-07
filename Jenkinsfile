pipeline {
    agent any

    environment {
        APP_NAME = "ginchat"
        TARGET_HOST = "192.168.74.128"
        TARGET_USER = "root"
        TARGET_PATH = "/home/muyu/deploy"
    }

    stages {

        stage('Checkout') {
            steps {
                cleanWs()       // 清空工作区
                git(
                    branch: 'master',
                    url: 'git@github.com:muyu0211/GoChat.git',
                    credentialsId: 'server-ssh-key'
                )
            }
        }

        stage('Build') {
            steps {
                script {
                    docker.image('golang:1.24.5').inside {
                        sh '''
                        go mod tidy
                        CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ${APP_NAME} ./cmd
                        '''
                    }
                }

                sh '''
                md5sum ${APP_NAME}
                '''
            }
        }

        stage('Deploy') {
            steps {
                sshagent(['server-ssh-key']) {
                    sh '''
                    md5sum ${APP_NAME}

                    # 停止当前服务
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "
                        pkill -f ${APP_NAME} || true
                    "

                    # 上传文件
                    scp -o StrictHostKeyChecking=no ${APP_NAME} ${TARGET_USER}@${TARGET_HOST}:${TARGET_PATH}/

                    # 启动服务
                    ssh -o StrictHostKeyChecking=no ${TARGET_USER}@${TARGET_HOST} "
                        cd ${TARGET_PATH}
                        chmod +x ./${APP_NAME}
                        nohup ./${APP_NAME} > server.log 2>&1 &
                    "
                    '''
                }
            }
        }
    }
}