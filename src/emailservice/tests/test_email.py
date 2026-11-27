import demo_pb2
from email_server import DummyEmailService, template


# Test 1: Email Service 正常回傳
def test_send_confirmation_success():
    service = DummyEmailService()

    request = demo_pb2.SendOrderConfirmationRequest(
        email="test@example.com",
        order=demo_pb2.OrderResult(order_id="123")
    )

    response = service.SendOrderConfirmation(request, None)

    assert isinstance(response, demo_pb2.Empty)


# Test 2: Email Template 包含正確訂單編號
def test_email_contains_order_id():
    order = demo_pb2.OrderResult(
        order_id="ORDER-123"
    )

    html = template.render(order=order)

    assert "#ORDER-123" in html