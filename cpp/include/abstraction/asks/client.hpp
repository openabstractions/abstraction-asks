#pragma once
#include <abstraction/asks/api/rec.h>
#include <abstraction/ipc/frame.hpp>
#include <algorithm>
namespace abstraction::asks {
inline void validate(const api::QuestionObservation& result){
 if(std::find(api::kObservationOutcomeNames.begin(),api::kObservationOutcomeNames.end(),result.outcome)==api::kObservationOutcomeNames.end())throw api::ServiceError("invalid_response","unknown outcome");
 const bool pending=result.outcome=="pending",answered=result.outcome=="answered";
 if((pending||answered)!=result.answer.has_value())throw api::ServiceError("invalid_observation","inconsistent answer presence");
 if(result.answer){const auto& a=*result.answer;
  if(a.id.empty()||a.pending!=pending||(pending&&(!a.option.empty()||a.yes||a.kept))||(answered&&a.option.empty()))throw api::ServiceError("invalid_observation","inconsistent answer");
 }
}
class Client {
public:
 explicit Client(std::string endpoint):transport_(std::move(endpoint),5000,1u<<20){}
 Client(std::string endpoint,ipc::Deadline deadline):transport_(std::move(endpoint),deadline,1u<<20){}
 Client with_server_expectation(std::optional<ipc::ServerExpectation> server) const {auto copy=*this;copy.transport_=transport_.with_server_expectation(std::move(server));return copy;}
 Client with_cancellation(ipc::CancellationToken token)const{auto copy=*this;copy.transport_=transport_.with_cancellation(std::move(token));return copy;}
 api::QuestionObservation ask(const api::ApplicationQuestion& question)const{
  auto transport=transport_;api::QuestionApplicationClient<ipc::FrameTransport> client(transport);auto result=client.ask(question);validate(result);return result;
 }
 api::QuestionObservation observe(const std::string& request_key,std::int64_t wait_ms=0)const{
  if(wait_ms<0||wait_ms>30000)throw api::ServiceError("invalid_request","wait_ms must be 0..30000");
  auto transport=transport_;api::QuestionApplicationClient<ipc::FrameTransport> client(transport);auto result=client.observe(request_key,wait_ms);validate(result);return result;
 }
private: ipc::FrameTransport transport_;
};
}
