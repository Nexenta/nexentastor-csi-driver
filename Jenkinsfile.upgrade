node('master') {
    docker.withServer('unix:///var/run/docker.sock') {
        stage('Git clone') {
            git url: 'https://github.com/Nexenta/nexentastor-csi-driver.git', branch: 'master'
        }
        stage('Tests [unit]') {
            docker
                .image('solutions-team-jenkins-agent-ubuntu')
                .inside('--volumes-from solutions-team-jenkins-master') {
                    sh "make test-unit-container"
                }
        }
        stage('Tests [e2e-ns]') {
            docker
                .image('solutions-team-jenkins-agent-ubuntu')
                .inside('--volumes-from solutions-team-jenkins-master') {
                    sh '''
                        export NOCOLORS=true
                        make test-e2e-ns-container
                    '''
                }
        }
        stage('Tests [csi-sanity]') {
            docker
                .image('solutions-team-jenkins-agent-ubuntu')
                .inside('--volumes-from solutions-team-jenkins-master') {
                    sh "make test-csi-sanity-container"
                }
        }
        stage('Build') {
            docker
                .image('solutions-team-jenkins-agent-ubuntu')
                .inside('--volumes-from solutions-team-jenkins-master') {
                    sh "make container-build"
                }
        }
        //TODO: restart Docker on .92 to apply changes to trusted hosts (/etc/docker/daemon.json)
        //stage('Push [local registry]') {
        //    docker
        //        .image('solutions-team-jenkins-agent-ubuntu')
        //        .inside('--volumes-from solutions-team-jenkins-master') {
        //            sh "make container-push-local"
        //        }
        //}
        //stage('Tests [local registry]') {
        //    docker
        //        .image('solutions-team-jenkins-agent-ubuntu')
        //        .inside('--volumes-from solutions-team-jenkins-master') {
        //            sh '''
        //                export NOCOLORS=true
        //                make test-e2e-k8s-local-image-container
        //            '''
        //        }
        //}
        stage('Push [hub.docker.com]') {
            docker
                .image('solutions-team-jenkins-agent-ubuntu')
                .inside('--volumes-from solutions-team-jenkins-master') {
                    withCredentials([usernamePassword(
                        credentialsId: 'hub.docker-nedgeui',
                        passwordVariable: 'DOCKER_PASS',
                        usernameVariable: 'DOCKER_USER'
                    )]) {
                        sh '''
                            docker login -u ${DOCKER_USER} -p ${DOCKER_PASS};
                            make container-push-remote
                        '''
                    }
                }
        }
        stage('Tests [k8s hub.docker.com]') {
            docker
                .image('solutions-team-jenkins-agent-ubuntu')
                .inside('--volumes-from solutions-team-jenkins-master') {
                    sh '''
                        export NOCOLORS=true
                        make test-e2e-k8s-remote-image-container
                    '''
                }
        }
    }
}
