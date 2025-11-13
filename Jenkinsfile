pipeline {
  agent any

  environment {
    APP_NAME = "client-nest"
    APP_DIR = "/home/ubuntu/client-nest"
  }

  stages {
    stage('Checkout') {
      steps {
        git branch: "${env.GIT_BRANCH}", url: 'https://github.com/manishagolane/client-nest.git'
      }
    }

    stage('Build') {
      steps {
        sh '''
          echo "Building ${APP_NAME}..."
          go mod tidy
          go build -o ${APP_NAME}
        '''
      }
    }

    stage('Deploy') {
      steps {
        sshagent (credentials: ['ec2-deploy-key']) {
          sh '''
            echo "Deploying ${APP_NAME} on EC2..."
            scp ${APP_NAME} ubuntu@16.16.64.246:${APP_DIR}/
            ssh ubuntu@16.16.64.246 "sudo systemctl restart ${APP_NAME}.service && sudo systemctl status ${APP_NAME}.service --no-pager"
          '''
        }
      }
    }
  }

  post {
    success {
      echo "${APP_NAME} successfully built and deployed!"
    }
    failure {
      echo "Build or deploy failed!"
    }
  }
}
